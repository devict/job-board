package data

import (
	"testing"
)

func TestJobValidate(t *testing.T) {
	testJob := &NewJob{
		Position:     "test position",
		Organization: "test org",
		Url:          "https://test.com/",
		Email:        "test@test.com",
	}

	// test valid url format
	result := testJob.Validate(false)
	if result["url"] == "Must provide a valid Url" {
		t.Error("valid url, should have no error - result was=", result["url"])
	}

	// test valid email format
	result = testJob.Validate(false)
	if result["email"] == "Must provide a valid Email" {
		t.Error("valid email, should have no error - result was=", result["email"])
	}

	// test bad url format
	testJob.Url = "https//test.com/"
	result = testJob.Validate(false)
	if result["url"] != "Must provide a valid Url" {
		t.Error("bad url, should show an error - result was=", result["url"])
	}

	// test bad email format
	testJob.Email = "testtest.com"
	result = testJob.Validate(false)
	if result["email"] != "Must provide a valid Email" {
		t.Error("bad email, should show an error - result was=", result["email"])
	}
}

func TestRoleValidate(t *testing.T) {
	testRole := &NewRole{
		Name:     "test testington",
		Email:    "test@foobar.com",
		Phone:    "316-555-5555",
		Role:     "any",
		Resume:   "#wow\n\nhire this person",
		Linkedin: "https://linkedin.com/in/testtestingtonsupreme",
		Website:  "https://www.example.com",
		Github:   "https://www.github.com/example",
		CompLow:  "10,000",
		CompHigh: "1,000,000",
	}

	// test valid name
	result := testRole.Validate(false)
	if result["name"] == "Must provide a Name" {
		t.Error("valid name, should have no error - result was=", result["name"])
	}

	// test valid email format
	if result["email"] == "Must provide a valid Email" {
		t.Error("valid email, should have no error - result was=", result["email"])
	}

	// test valid role
	if result["role"] == "Must provide a Role" {
		t.Error("valid email, should have no error - result was=", result["role"])
	}

	// test valid role
	if result["resume"] == "Must provide a Resume" {
		t.Error("valid email, should have no error - result was=", result["resume"])
	}

	// test valid urls
	if result["linkedin"] == "Must provide a valid Url" {
		t.Error("valid url, should have no error - result was=", result["linkedin"])
	}
	if result["website"] == "Must provide a valid Url" {
		t.Error("valid url, should have no error - result was=", result["website"])
	}
	if result["github"] == "Must provide a valid Url" {
		t.Error("valid url, should have no error - result was=", result["github"])
	}

	// test bad url format
	testRole.Linkedin = "https//test.com/"
	testRole.Website = "https//test.com/"
	testRole.Github = "https//test.com/"
	result = testRole.Validate(false)
	if result["linkedin"] != "Must provide a valid Url" {
		t.Error("bad url, should show an error - result was=", result["linkedin"])
	}
	if result["website"] != "Must provide a valid Url" {
		t.Error("bad url, should show an error - result was=", result["website"])
	}
	if result["github"] != "Must provide a valid Url" {
		t.Error("bad url, should show an error - result was=", result["github"])
	}

	// test bad email format
	testRole.Email = "testtest.com"
	result = testRole.Validate(false)
	if result["email"] != "Must provide a valid Email" {
		t.Error("bad email, should show an error - result was=", result["email"])
	}
}
