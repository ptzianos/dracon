package pipelines

import (
	"context"
	"fmt"
	"slices"

	"github.com/go-errors/errors"
	tektonv1beta1api "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	kustomizetypes "sigs.k8s.io/kustomize/api/types"

	"github.com/ocurity/dracon/pkg/components"
	"github.com/ocurity/dracon/pkg/manifests"
)

var _ Backend[*tektonv1beta1api.Pipeline] = (*tektonV1Beta1Backend)(nil)

var (
	// ErrNoComponentsInKustomization is returned when a kustomization has no components listed
	ErrNoComponentsInKustomization = errors.New("no components listed in kustomization")
	// ErrKustomizationMissingBaseResources is returned when a kustomization doesn't have 2 base resources
	ErrKustomizationMissingBaseResources = errors.New("kustomization must have exactly 2 resources: a base pipeline and a base task")
	// ErrNoTasks is returned when no tasks are provided to the Tekton backend
	ErrNoTasks = errors.New("no tasks provided")
)

// Kustomization is a wrapper around the `Kustomization` struct of kustomize that adds some fields
// and methods to the object for parsing.
type Kustomization struct {
	*kustomizetypes.Kustomization
	// KustomizationDir is the relative path to the directory where the kustomization lives
	KustomizationDir string
}

type tektonV1Beta1Backend struct {
	pipeline *tektonv1beta1api.Pipeline
	tasks    []*tektonv1beta1api.Task
	suffix   string
}

// func renameParameterRef(oldVal string, newParameterNames map[string]string) string {
// 	if !strings.HasPrefix(oldVal, "$(params.") {
// 		return oldVal
// 	}
// 	oldParamRef := strings.Split(oldVal, "$(params.")[1]
// 	// remove last parentheses
// 	oldParamRef = oldParamRef[:len(oldParamRef)-1]
// 	return fmt.Sprintf("$(params.%s)", newParameterNames[oldParamRef])
// }

// fixTaskPrefixSuffix adds a prefix and a suffix to the name of the task and all the task
// parameters. Having task parameters prefixed with the same name as the task itself, helps
// users figure out more easily which parameters configure what.
// func fixTaskPrefixSuffix(task tektonV1Beta1.Task, prefix, suffix string) {
// 	// keep track of renamings so that we can also fix the environment variables
// 	// referencing the parameters
// 	newParameterNames := map[string]string{}
// 	for _, param := range task.Spec.Parameters {
// 		oldParamName := param.Name
// 		paramNameChunks := strings.Split(param.Name, task.Name)
// 		param.Name = prefix + task.Name + suffix + paramNameChunks[1]
// 		newParameterNames[oldParamName] = param.Name
// 	}
// 	// fix references to parameters in step env vars and images
// 	for _, step := range task.Spec.Steps {
// 		for _, env := range step.Env {
// 			env.Value = renameParameterRef(env.Value, newParameterNames)
// 		}
// 		step.Image = renameParameterRef(step.Image, newParameterNames)
// 	}
// 	task.Name = prefix + task.Name + suffix
// }

// addAnchorResult adds an `anchor` entry to the results section of a Task. This helps reduce the
// amount of boilerplate needed to be written by a user to introduce a component.
func addAnchorResult(task *tektonv1beta1api.Task) {
	if task.Labels[components.LabelKey] == components.Consumer.String() || task.Labels[components.LabelKey] == components.Base.String() {
		return
	}

	task.Spec.Results = append(task.Spec.Results, tektonv1beta1api.TaskResult{
		Name:        "anchor",
		Description: "An anchor to allow other tasks to depend on this task.",
	})

	task.Spec.Steps = append(task.Spec.Steps, tektonv1beta1api.Step{
		Name:   "anchor",
		Image:  "docker.io/busybox",
		Script: "echo \"$(context.task.name)\" > \"$(results.anchor.path)\"",
	})
}

// addAnchorParameter adds an `anchors` entry to the parameters of a Task. This entry will then be
// filled in the pipeline with the anchors of the tasks that this task depends on.
func addAnchorParameter(task *tektonv1beta1api.Task) {
	componentType, err := components.ToComponentType(task.Labels[components.LabelKey])
	if err != nil {
		panic(errors.Errorf("%s: %w", task.Name, err))
	}
	if componentType < components.Producer {
		return
	}

	for _, param := range task.Spec.Params {
		if param.Name == "anchors" {
			return
		}
	}

	task.Spec.Params = append(task.Spec.Params, tektonv1beta1api.ParamSpec{
		Name:        "anchors",
		Description: "A list of tasks that this task depends on",
		Type:        "array",
		Default: &tektonv1beta1api.ParamValue{
			Type: tektonv1beta1api.ParamTypeArray,
		},
	})
}

// ResolveKustomizationResources checks the resources section to find the base pipeline and
// task and fetches them from wherever they are located.
func (pk *Kustomization) ResolveKustomizationResources(ctx context.Context) (*tektonv1beta1api.Pipeline, []*tektonv1beta1api.Task, error) {
	var err error
	var baseTaskPath string
	var basePipeline *tektonv1beta1api.Pipeline

	if basePipeline, err = manifests.LoadTektonV1Beta1Pipeline(ctx, pk.KustomizationDir, pk.Resources[0]); err != nil {
		if basePipeline, err = manifests.LoadTektonV1Beta1Pipeline(ctx, pk.KustomizationDir, pk.Resources[1]); err != nil {
			return nil, nil, err
		}
		baseTaskPath = pk.Resources[0]
	} else {
		baseTaskPath = pk.Resources[1]
	}

	baseTask, err := manifests.LoadTektonV1Beta1Task(ctx, pk.KustomizationDir, baseTaskPath)
	if err != nil {
		return nil, nil, errors.Errorf("%s: could not load task: %w", baseTaskPath, err)
	}

	if len(pk.Components) == 0 {
		return nil, nil, errors.Errorf("%s: %w", pk.KustomizationDir, ErrNoComponentsInKustomization)
	}

	taskList := []*tektonv1beta1api.Task{baseTask}
	for _, pathOrURI := range pk.Components {
		newTask, err := manifests.LoadTektonV1Beta1Task(ctx, pk.KustomizationDir, pathOrURI)
		if err != nil {
			return nil, nil, err
		}

		if err = components.ValidateTask(newTask); err != nil {
			return nil, nil, errors.Errorf("%s: invalid task found: %w", newTask.Name, err)
		}

		newTask.Namespace = pk.Namespace
		taskList = append(taskList, newTask)
	}

	return basePipeline, taskList, nil
}

// NewTektonV1Beta1Backend returns an implementation of the Backend interface
// that will produce a Tekton Pipeline object with all the configured tasks.
func NewTektonV1Beta1Backend(basePipeline *tektonv1beta1api.Pipeline, tasks []*tektonv1beta1api.Task, suffix string) (Backend[*tektonv1beta1api.Pipeline], error) {
	if len(tasks) == 0 {
		return nil, errors.Errorf("%w", ErrNoTasks)
	}

	tektonBackend := &tektonV1Beta1Backend{pipeline: basePipeline, tasks: tasks[:], suffix: suffix}
	for _, task := range tasks {
		// TODO(?): revisit if we need this in the future
		// fixTaskPrefixSuffix(task, prefix, suffix)
		addAnchorParameter(task)
		addAnchorResult(task)
	}

	// Sort tasks based on their component type
	slices.SortFunc(tektonBackend.tasks, func(a *tektonv1beta1api.Task, b *tektonv1beta1api.Task) int {
		componentTypeA := components.MustGetComponentType(a.Labels[components.LabelKey])
		componentTypeB := components.MustGetComponentType(b.Labels[components.LabelKey])
		return int(componentTypeA) - int(componentTypeB)
	})

	return tektonBackend, nil
}

func (tb *tektonV1Beta1Backend) Generate() (*tektonv1beta1api.Pipeline, error) {
	tb.pipeline.Name = tb.pipeline.Name + tb.suffix
	pipelineWorkspaces := map[string]struct{}{}
	anchors := map[string][]string{}

	for _, task := range tb.tasks {
		componentType := task.Labels[components.LabelKey]
		anchors[componentType] = append(anchors[componentType], task.Name)

		// add task to pipeline tasks
		pipelineTask := tektonv1beta1api.PipelineTask{
			Name: task.Name,
			TaskRef: &tektonv1beta1api.TaskRef{
				Name: task.Name,
			},
		}

		// add task's workspaces to pipeline workspaces
		// make sure to propagate the `optional` field
		for _, ws := range task.Spec.Workspaces {
			if _, inserted := pipelineWorkspaces[ws.Name]; !inserted {
				tb.pipeline.Spec.Workspaces = append(tb.pipeline.Spec.Workspaces, tektonv1beta1api.PipelineWorkspaceDeclaration{
					Name:     ws.Name,
					Optional: ws.Optional,
				})
				pipelineWorkspaces[ws.Name] = struct{}{}
			}
			pipelineTask.Workspaces = append(pipelineTask.Workspaces, tektonv1beta1api.WorkspacePipelineTaskBinding{
				Name:      ws.Name,
				Workspace: ws.Name,
			})
		}

		// add the task's parameters to the pipeline's parameters and
		// reference them in the pipeline task parameters
		pipelineTask.Params = make(tektonv1beta1api.Params, len(task.Spec.Params))

		for i, param := range task.Spec.Params {
			pipelineTask.Params[i] = tektonv1beta1api.Param{
				Name:  param.Name,
				Value: tektonv1beta1api.ParamValue{},
			}

			if param.Name == "anchors" {
				anchorTargetComponentType := components.MustGetComponentType(componentType) - 1
				values := []string{}

				// get all the tasks that should be finished before this one starts
				for _, anchorTarget := range anchors[anchorTargetComponentType.String()] {
					values = append(values, fmt.Sprintf("$(tasks.%s.results.anchor)", anchorTarget))
				}

				pipelineTask.Params[i].Value.ArrayVal = values
				pipelineTask.Params[i].Value.Type = tektonv1beta1api.ParamTypeArray
			} else {
				switch param.Type {
				case tektonv1beta1api.ParamTypeArray:
					pipelineTask.Params[i].Value.Type = param.Type
					pipelineTask.Params[i].Value.ArrayVal = []string{fmt.Sprintf("$(params.%s)", param.Name)}
				case tektonv1beta1api.ParamTypeString:
					pipelineTask.Params[i].Value.Type = param.Type
					pipelineTask.Params[i].Value.StringVal = fmt.Sprintf("$(params.%s)", param.Name)
				case "":
					return nil, errors.Errorf("parameter %s of task %s has no type set", param.Name, task.Name)
				}

				// ensure that the parameter type is always set
				if param.Default != nil && param.Default.Type == "" {
					param.Default.Type = param.Type
				}

				// add parameter to pipeline parameters
				tb.pipeline.Spec.Params = append(tb.pipeline.Spec.Params, tektonv1beta1api.ParamSpec{
					Name:        param.Name,
					Type:        param.Type,
					Description: param.Description,
					Default:     param.Default,
				})
			}
		}

		// add scan ID and scan time to all producers
		if task.Labels[components.LabelKey] == components.Producer.String() {
			addParamsAndEnvVars(&pipelineTask, anchors, task)
		}

		// add task reference to pipeline's tasks
		tb.pipeline.Spec.Tasks = append(tb.pipeline.Spec.Tasks, pipelineTask)
	}

	return tb.pipeline, nil
}

// addParamsAndEnvVars will add parameters and environment variables to the producer task that will
// allow it to pick the start time, pipeline UUID and any tags that have been given as parameter to
// the pipeline so that the issues discovered can be annotated with these values.
func addParamsAndEnvVars(pipelineTask *tektonv1beta1api.PipelineTask, anchors map[string][]string, task *tektonv1beta1api.Task) {
	pipelineTask.Params = append(pipelineTask.Params, []tektonv1beta1api.Param{
		{
			Name: "dracon_scan_id",
			Value: tektonv1beta1api.ParamValue{
				Type:      tektonv1beta1api.ParamTypeString,
				StringVal: fmt.Sprintf("$(tasks.%s.results.dracon-scan-id)", anchors[components.Base.String()][0]),
			},
		},
		{
			Name: "dracon_scan_start_time",
			Value: tektonv1beta1api.ParamValue{
				Type:      tektonv1beta1api.ParamTypeString,
				StringVal: fmt.Sprintf("$(tasks.%s.results.dracon-scan-start-time)", anchors[components.Base.String()][0]),
			},
		},
		{
			Name: "dracon_scan_tags",
			Value: tektonv1beta1api.ParamValue{
				Type:      tektonv1beta1api.ParamTypeString,
				StringVal: fmt.Sprintf("$(tasks.%s.results.dracon-scan-tags)", anchors[components.Base.String()][0]),
			},
		},
	}...)

	task.Spec.Params = append(task.Spec.Params, tektonv1beta1api.ParamSpecs{
		{
			Name: "dracon_scan_id",
			Type: tektonv1beta1api.ParamTypeString,
		},
		{
			Name: "dracon_scan_start_time",
			Type: tektonv1beta1api.ParamTypeString,
		},
		{
			Name: "dracon_scan_tags",
			Type: tektonv1beta1api.ParamTypeString,
		},
	}...)

	for i, step := range task.Spec.Steps {
		step.Env = append(step.Env, []corev1.EnvVar{
			{
				Name:  "DRACON_SCAN_TIME",
				Value: "$(params.dracon_scan_start_time)",
			},
			{
				Name:  "DRACON_SCAN_ID",
				Value: "$(params.dracon_scan_id)",
			},
			{
				Name:  "DRACON_SCAN_TAGS",
				Value: "$(params.dracon_scan_tags)",
			},
		}...)
		task.Spec.Steps[i] = step
	}
}
