package reporters

import (
	"encoding/xml"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/testrunner"
)

func init() {
	testrunner.RegisterReporter(XUnitReporter, ReportXUnit)
}

const (
	XUnitReporter testrunner.TestReporter = "xUnit"
)

type testSuites struct {
	XMLName xml.Name    `xml:"testsuites"`
	Suites  []testSuite `xml:"testsuite"`
}
type testSuite struct {
	Suites []testSuite `xml:"testsuite,omitempty"`
	Cases  []testCase  `xml:"testcase,omitempty"`
}
type testCase struct {
	Name string        `xml:"Name,attr"`
	Time time.Duration `xml:"Time,attr"`

	Error   string `xml:"Error,omitempty"`
	Failure string `xml:"Failure,omitempty"`
}

func ReportXUnit(results []testrunner.TestResult) (string, error) {
	// package => data stream => test cases
	packages := map[string]map[string][]testCase{}

	var numPackages int

	for _, r := range results {
		if _, exists := packages[r.Package]; !exists {
			packages[r.Package] = map[string][]testCase{}
			numPackages++
		}

		if _, exists := packages[r.Package][r.DataStream]; !exists {
			packages[r.Package][r.DataStream] = make([]testCase, 1)
		}

		c := testCase{
			Name:    r.Name,
			Time:    r.TimeTaken,
			Error:   r.ErrorMsg,
			Failure: r.FailureMsg,
		}

		packages[r.Package][r.DataStream] = append(packages[r.Package][r.DataStream], c)
	}

	var ts testSuites
	ts.Suites = make([]testSuite, numPackages)

	for _, pkg := range packages {
		pkgSuite := testSuite{
			Suites: make([]testSuite, 1),
		}

		for _, ds := range pkg {
			dsSuite := testSuite{
				Cases: ds,
			}

			pkgSuite.Suites = append(pkgSuite.Suites, dsSuite)
		}

		ts.Suites = append(ts.Suites, pkgSuite)
	}

	out, err := xml.MarshalIndent(&ts, "", "  ")
	if err != nil {
		return "", errors.Wrap(err, "unable to format test results as xUnit")
	}

	return xml.Header + string(out), nil
}
