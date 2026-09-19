package domain

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ValidateCompletion checks the invariants shared by every terminal publisher.
// Definition-dependent node paths and output size are checked by the worker.
func ValidateCompletion(c ExecutionCompletion) error {
	if !c.Status.Terminal() {
		return errors.New("completion requires terminal status")
	}
	report := c.Report
	if report.Errors == nil || report.Roots == nil {
		return errors.New("terminal report arrays must not be null")
	}
	summary := report.ErrorSummary
	if summary.Returned != uint64(len(report.Errors)) || summary.Total < summary.Returned || summary.Truncated != (summary.Returned < summary.Total) {
		return errors.New("inconsistent terminal error summary")
	}
	available, allComplete, err := validateRootOutcomes(report.Roots)
	if err != nil {
		return err
	}
	if c.Status == Failed {
		if c.Error == nil || c.Error.Code == "" || c.Error.Message == "" || len(c.Data) != 0 {
			return errors.New("failed completion requires global failure and no result")
		}
		return nil
	}
	if c.Error != nil || !available {
		return errors.New("successful completion requires available root and no global failure")
	}
	if c.Status == Completed && (summary.Total != 0 || !allComplete) {
		return errors.New("completed execution cannot contain errors or incomplete roots")
	}
	if c.Status == CompletedWithErrors && summary.Total == 0 {
		return errors.New("partial completion requires a final diagnostic")
	}
	if c.APIVersion == 2 {
		if len(c.Data) != 0 {
			return errors.New("v2 completion must not contain an assembled result")
		}
		return nil
	}
	var data map[string]json.RawMessage
	if err := json.Unmarshal(c.Data, &data); err != nil || data == nil {
		return errors.New("terminal result must be a JSON object")
	}
	if len(data) != len(report.Roots) {
		return errors.New("result keys do not match root outcomes")
	}
	for _, root := range report.Roots {
		if _, exists := data[root.Name]; !exists {
			return fmt.Errorf("result is missing declared root %q", root.Name)
		}
	}
	return nil
}

func validateRootOutcomes(roots []RootOutcome) (available, allComplete bool, err error) {
	names := make(map[string]struct{}, len(roots))
	allComplete = true
	for _, root := range roots {
		if root.Name == "" {
			return false, false, errors.New("root name must not be empty")
		}
		if _, duplicate := names[root.Name]; duplicate {
			return false, false, errors.New("root names must be unique")
		}
		names[root.Name] = struct{}{}
		switch root.Status {
		case Completed, CompletedWithErrors:
			if !root.DataAvailable {
				return false, false, errors.New("completed root must have available data")
			}
		case Failed, ExecutionStatus("not_started"):
			if root.DataAvailable {
				return false, false, errors.New("failed or unstarted root cannot have available data")
			}
		default:
			return false, false, errors.New("invalid terminal root status")
		}
		available = available || root.DataAvailable
		allComplete = allComplete && root.Status == Completed
	}
	return available, allComplete, nil
}
