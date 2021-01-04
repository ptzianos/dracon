package jira

import (
	"testing"

	"github.com/andygrunwald/go-jira"
	"github.com/stretchr/testify/assert"
	"github.com/trivago/tgo/tcontainer"
)

func TestSetDefaultFields(t *testing.T) {
	res := getDefaultFields(sampleConfig)

	exp := defaultJiraFields{
		Project: jira.Project{
			Key: "TOY",
		},
		IssueType: jira.IssueType{
			Name: "Vulnerability",
		},
		Components: []*jira.Component{
			&jira.Component{Name: "c1"},
			&jira.Component{Name: "c2"},
			&jira.Component{Name: "c3"},
		},
		AffectsVersions: []*jira.AffectsVersion{
			&jira.AffectsVersion{Name: "V1"},
			&jira.AffectsVersion{Name: "V2"},
		},
		Labels: []string(nil),
		CustomFields: tcontainer.MarshalMap{
			"customfield_10000": []map[string]string{{"value": "foo"}, {"value": "bar"}},
		},
	}

	assert.EqualValues(t, res, exp)
}

func TestMakeCustomField(t *testing.T) {
	res1 := makeCustomField("single-value", []string{"test-value"})
	exp1 := map[string]string{"value": "test-value"}

	res2 := makeCustomField("multi-value", []string{"value1", "value2", "value3"})
	exp2 := []map[string]string{
		{"value": "value1"},
		{"value": "value2"},
		{"value": "value3"},
	}

	res3 := makeCustomField("float", []string{"4.22"})
	exp3 := 4.22

	res4 := makeCustomField("simple-value",[]string{"test-value"})
	exp4 := "test-value"

	assert.EqualValues(t, res1, exp1)
	assert.EqualValues(t, res2, exp2)
	assert.Equal(t, res3, exp3)
	assert.Equal(t, res4, exp4)
}

func TestMakeDescription(t *testing.T) {
	extras := []string{"tool_name", "target", "confidence_text"}
	res := makeDescription(sampleResult, extras)
	exp := "This issue was automatically generated by the Dracon security pipeline.\n\n" +
		"*this is a test description*\n\n" +
		"{code:}\n" +
		"tool_name:                 spotbugs\n" +
		"target:                    //foo1/bar1:baz2\n" +
		"confidence_text:           Info\n" +
		"{code}\n"
	assert.Equal(t, res, exp)
}

func TestMakeSummary(t *testing.T) {
	res := makeSummary(sampleResult)
	exp := "bar1:baz2 Unit Test Title"

	assert.Equal(t, res, exp)
}
