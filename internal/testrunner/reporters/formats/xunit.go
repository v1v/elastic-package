// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package formats

import (
	"encoding/xml"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/testrunner"
)

func init() {
	testrunner.RegisterReporterFormat(ReportFormatXUnit, reportXUnitFormat)
}

const (
	// ReportFormatXUnit reports test results in the xUnit format
	ReportFormatXUnit testrunner.TestReportFormat = "xUnit"
)

type testSuites struct {
	XMLName xml.Name    `xml:"testsuites"`
	Suites  []testSuite `xml:"testsuite"`
}
type testSuite struct {
	Comment string `xml:",comment"`

	Name        string `xml:"name,attr"`
	NumTests    int    `xml:"tests,attr,omitempty"`
	NumFailures int    `xml:"failures,attr,omitempty"`
	NumErrors   int    `xml:"errors,attr,omitempty"`

	Suites []testSuite `xml:"testsuite,omitempty"`
	Cases  []testCase  `xml:"testcase,omitempty"`
}
type testCase struct {
	Name      string        `xml:"name,attr"`
	ClassName string        `xml:"classname,attr`
	Time      time.Duration `xml:"time,attr"`

	Error   string `xml:"error,omitempty"`
	Failure string `xml:"failure,omitempty"`
}

func reportXUnitFormat(results []testrunner.TestResult) (string, error) {
	// test type => package => data stream => test cases
	tests := map[string]map[string]map[string][]testCase{}

	var numTests, numFailures, numErrors int
	for _, r := range results {
		testType := string(r.TestType)
		if _, exists := tests[testType]; !exists {
			tests[testType] = map[string]map[string][]testCase{}
		}

		if _, exists := tests[testType][r.Package]; !exists {
			tests[testType][r.Package] = map[string][]testCase{}
		}

		if _, exists := tests[testType][r.Package][r.DataStream]; !exists {
			tests[testType][r.Package][r.DataStream] = make([]testCase, 0)
		}

		var failure string
		if r.FailureMsg != "" {
			failure = r.FailureMsg
			numFailures++
		}

		if r.FailureDetails != "" {
			failure += ": " + r.FailureDetails
		}

		if r.ErrorMsg != "" {
			numErrors++
		}

		c := testCase{
			Name:    r.Name,
			Time:    r.TimeElapsed,
			Error:   r.ErrorMsg,
			Failure: failure,
		}
		numTests++

		tests[testType][r.Package][r.DataStream] = append(tests[testType][r.Package][r.DataStream], c)
	}

	var ts testSuites
	ts.Suites = make([]testSuite, 0)

	for testType, packages := range tests {
		testTypeSuite := testSuite{
			Comment: fmt.Sprintf("test suite for %s tests", testType),
			Name:    testType,

			NumTests:    numTests,
			NumFailures: numFailures,
			NumErrors:   numErrors,

			Suites: make([]testSuite, 0),
		}

		for pkgName, pkg := range packages {
			pkgSuite := testSuite{
				Name:    pkgName,
				Comment: fmt.Sprintf("test suite for package: %s", pkgName),
				Suites:  make([]testSuite, 0),
			}

			for dsName, ds := range pkg {
				dsSuite := testSuite{
					Name:    dsName,
					Comment: fmt.Sprintf("test suite for data stream: %s", dsName),
					Cases:   ds,
				}

				pkgSuite.Suites = append(pkgSuite.Suites, dsSuite)
			}

			testTypeSuite.Suites = append(testTypeSuite.Suites, pkgSuite)
		}

		ts.Suites = append(ts.Suites, testTypeSuite)
	}

	out, err := xml.MarshalIndent(&ts, "", "  ")
	if err != nil {
		return "", errors.Wrap(err, "unable to format test results as xUnit")
	}

	return xml.Header + string(out), nil
}
