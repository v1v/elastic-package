// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipeline

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/testrunner"
)

const (
	// TestType defining pipeline tests
	TestType testrunner.TestType = "pipeline"
)

type runner struct {
	options testrunner.TestOptions
}

// Run runs the pipeline tests defined under the given folder
func Run(options testrunner.TestOptions) ([]testrunner.TestResult, error) {
	r := runner{options}
	return r.run()
}

func (r *runner) run() ([]testrunner.TestResult, error) {
	testCaseFiles, err := r.listTestCaseFiles()
	if err != nil {
		return nil, errors.Wrap(err, "listing test case definitions failed")
	}

	dataStreamPath, found, err := packages.FindDataStreamRootForPath(r.options.TestFolder.Path)
	if err != nil {
		return nil, errors.Wrap(err, "locating data_stream root failed")
	}
	if !found {
		return nil, errors.New("data stream root not found")
	}

	entryPipeline, pipelineIDs, err := installIngestPipelines(r.options.ESClient, dataStreamPath)
	if err != nil {
		return nil, errors.Wrap(err, "installing ingest pipelines failed")
	}
	defer func() {
		err := uninstallIngestPipelines(r.options.ESClient, pipelineIDs)
		if err != nil {
			logger.Warnf("uninstalling ingest pipelines failed: %v", err)
		}
	}()

	var failed bool
	results := make([]testrunner.TestResult, 0)
	for _, testCaseFile := range testCaseFiles {
		tr := testrunner.TestResult{
			TestType:   TestType,
			Package:    r.options.TestFolder.Package,
			DataStream: r.options.TestFolder.DataStream,
		}
		startTime := time.Now()

		tc, err := r.loadTestCaseFile(testCaseFile)
		if err != nil {
			err := errors.Wrap(err, "loading test case failed")
			tr.ErrorMsg = err.Error()
			return results, err
		}
		fmt.Printf("Test case: %s\n", tc.name)
		tr.Name = tc.name
		results = append(results, tr)

		result, err := simulatePipelineProcessing(r.options.ESClient, entryPipeline, tc)
		if err != nil {
			err := errors.Wrap(err, "simulating pipeline processing failed")
			tr.ErrorMsg = err.Error()
			return results, err
		}

		tr.TimeTaken = time.Now().Sub(startTime)
		err = r.verifyResults(testCaseFile, result)
		if err == errTestCaseFailed {
			failed = true
			tr.FailureMsg = err.Error()
			continue
		}
		if err != nil {
			return results, errors.Wrap(err, "verifying test result failed")
		}
	}

	if failed {
		return results, errors.New("at least one test case failed")
	}

	return results, nil
}

func (r *runner) listTestCaseFiles() ([]string, error) {
	fis, err := ioutil.ReadDir(r.options.TestFolder.Path)
	if err != nil {
		return nil, errors.Wrapf(err, "reading pipeline tests failed (path: %s)", r.options.TestFolder.Path)
	}

	var files []string
	for _, fi := range fis {
		if strings.HasSuffix(fi.Name(), expectedTestResultSuffix) || strings.HasSuffix(fi.Name(), configTestSuffix) {
			continue
		}
		files = append(files, fi.Name())
	}
	return files, nil
}

func (r *runner) loadTestCaseFile(testCaseFile string) (*testCase, error) {
	testCasePath := filepath.Join(r.options.TestFolder.Path, testCaseFile)
	testCaseData, err := ioutil.ReadFile(testCasePath)
	if err != nil {
		return nil, errors.Wrapf(err, "reading input file failed (testCasePath: %s)", testCasePath)
	}

	var tc *testCase
	ext := filepath.Ext(testCaseFile)
	switch ext {
	case ".json":
		tc, err = createTestCaseForEvents(testCaseFile, testCaseData)
		if err != nil {
			return nil, errors.Wrapf(err, "creating test case for events failed (testCasePath: %s)", testCasePath)
		}
	case ".log":
		config, err := readConfigForTestCase(testCasePath)
		if err != nil {
			return nil, errors.Wrapf(err, "reading config for test case failed (testCasePath: %s)", testCasePath)
		}
		tc, err = createTestCaseForRawInput(testCaseFile, testCaseData, config)
		if err != nil {
			return nil, errors.Wrapf(err, "creating test case for events failed (testCasePath: %s)", testCasePath)
		}
	default:
		return nil, fmt.Errorf("unsupported extension for test case file (ext: %s)", ext)
	}
	return tc, nil
}

func (r *runner) verifyResults(testCaseFile string, result *testResult) error {
	testCasePath := filepath.Join(r.options.TestFolder.Path, testCaseFile)

	if r.options.GenerateTestResult {
		err := writeTestResult(testCasePath, result)
		if err != nil {
			return errors.Wrap(err, "writing test result failed")
		}
	}

	err := compareResults(testCasePath, result)
	if err == errTestCaseFailed {
		return errTestCaseFailed
	}
	if err != nil {
		return errors.Wrap(err, "comparing test results failed")
	}
	return nil
}

func init() {
	testrunner.RegisterRunner(TestType, Run)
}
