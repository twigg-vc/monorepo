package runnerlib

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
)

func parseValidateAndExpandCdFile(file []byte) (cdJobs []CdJob, ok bool, notOkErrMsg string) {
	// Parse the files into a list of the structs
	cdJobJsons, ok, notOkErrMsg := parseCdJobJsons(file)
	if !ok {
		return
	}
	for i := range cdJobJsons {
		cdJobJson := &cdJobJsons[i]
		// Cast to CiJobJsons to reuse logic
		ciJsons := make([]CiJobJson, len(cdJobJson.Stages))
		for j, stage := range cdJobJson.Stages {
			displayName := stage.Name
			if displayName == "" {
				displayName = fmt.Sprintf("%s-stage-%d", stage.Name, i)
			}
			ciJsons[j] = CiJobJson{
				Name:                displayName,
				ImageName:           stage.ImageName,
				Steps:               stage.Steps,
				TimeoutMilliSeconds: stage.TimeoutMilliSeconds,
				TimeoutSeconds:      stage.TimeoutSeconds,
				TimeoutMinutes:      stage.TimeoutMinutes,
			}
		}
		// Validate and expand the CI jobs
		pathPrefix := fmt.Sprintf("cd_jobs[%d].stages", i)
		if ok, notOkErrMsg = isValidCiJobJsons(pathPrefix, ciJsons); !ok {
			return nil, false, notOkErrMsg
		}
		if ok, notOkErrMsg = resolveJobPayloadTemplates(ciJsons, 0); !ok {
			return nil, false, notOkErrMsg
		}
		resolveTimeouts(ciJsons)
		// Now thaty the CI jobs are populated, put them back to the cd
		for j := range cdJobJson.Stages {
			cdJobJson.Stages[j].Name = ciJsons[j].Name
			cdJobJson.Stages[j].ImageName = ciJsons[j].ImageName
			cdJobJson.Stages[j].Steps = ciJsons[j].Steps
			cdJobJson.Stages[j].TimeoutMilliSeconds = ciJsons[j].TimeoutMilliSeconds
			cdJobJson.Stages[j].TimeoutSeconds = ciJsons[j].TimeoutSeconds
			cdJobJson.Stages[j].TimeoutMinutes = ciJsons[j].TimeoutMinutes
		}
	}
	// Final Conversion and High-Level Validation
	cdJobs = convertToCdJob(cdJobJsons)
	ok, notOkErrMsg = isValidCdJobs(cdJobs)
	return
}

func parseCdJobJsons(file []byte) (jsons []CdJobJson, ok bool, notOkErrMsg string) {
	// Try array form
	dec := json.NewDecoder(bytes.NewReader(file))
	dec.DisallowUnknownFields()
	arrayErr := decodeWithBetterError(dec, &jsons)
	if arrayErr == nil {
		if dec.More() {
			return nil, false, "trailing data after array"
		}
		return jsons, true, ""
	}
	// Try single object form
	dec = json.NewDecoder(bytes.NewReader(file))
	dec.DisallowUnknownFields()
	var single CdJobJson
	singleErr := decodeWithBetterError(dec, &single)
	if singleErr == nil {
		if dec.More() {
			return nil, false, "trailing data after object"
		}
		return []CdJobJson{single}, true, ""
	}
	return nil, false, pickBetterError(file, arrayErr, singleErr).Error()
}

func convertToCdJob(cdJobJsons []CdJobJson) (cdJobs []CdJob) {
	cdJobs = make([]CdJob, 0, len(cdJobJsons))
	for _, cdJson := range cdJobJsons {
		cdJob := CdJob{
			Name: cdJson.Name,
			On:   cdJson.On,
		}
		if len(cdJson.Stages) > 0 {
			cdJob.Stages = make([]CdJobPayload, 0, len(cdJson.Stages))
		}
		for i, stageJson := range cdJson.Stages {
			stageName := stageJson.Name
			if stageName == "" {
				stageName = fmt.Sprintf("%s-stage-%d", cdJson.Name, i)
			}
			cdJobPayload := CdJobPayload{
				CanAutoStart: stageJson.CanAutoStart,
				JobPayload: JobPayload{
					Name:                stageName,
					ImageName:           stageJson.ImageName,
					TimeoutMilliSeconds: stageJson.TimeoutMilliSeconds,
				},
			}
			if len(stageJson.Steps) > 0 {
				cdJobPayload.JobPayload.Steps = make([]JobStep, 0, len(stageJson.Steps))
			}
			for _, st := range stageJson.Steps {
				step := JobStep{Run: st.Run, Dir: st.Dir}
				if len(st.Env) > 0 {
					step.Env = st.Env
				}
				if len(st.Secrets) > 0 {
					step.Secrets = st.Secrets
				}
				cdJobPayload.JobPayload.Steps = append(cdJobPayload.JobPayload.Steps, step)
			}
			cdJob.Stages = append(cdJob.Stages, cdJobPayload)
		}
		cdJobs = append(cdJobs, cdJob)
	}
	return
}

func isValidCdJobs(cdJobs []CdJob) (bool, string) {
	if len(cdJobs) == 0 {
		return false, "cd jobs can't be empty"
	}
	for i, cdJob := range cdJobs {
		if cdJob.Name == "" {
			return false, fmt.Sprintf("cd_jobs[%d].Name: can't be empty", i)
		}
		if len(cdJob.Stages) == 0 {
			return false, fmt.Sprintf("cd_jobs[%d]: must have at least one stage", i)
		}
		if len(cdJob.Stages) > MaxCdJobStages {
			return false, fmt.Sprintf("cd_jobs[%d].Stages: too many stages (max %d)", i, MaxCdJobStages)
		}
		if len(cdJob.On) > MaxCdJobStages {
			return false, fmt.Sprintf("cd_jobs[%d].On: too many values (max %d) ", i, MaxCdJobStages)
		}
		for triggerI, trigger := range cdJob.On {
			if !slices.Contains(SupportedCdJobTriggers, trigger) {
				return false, fmt.Sprintf(
					"cd_jobs[%d].On[%d]: bad value %q (must be one of %v)",
					i, triggerI, trigger, SupportedCdJobTriggers)
			}
		}
		for j, stage := range cdJob.Stages {
			// Reuse the low-level payload validator
			ok, msg := isValidJobPayload(fmt.Sprintf("cd_jobs[%d].stages[%d]", i, j), stage.JobPayload)
			if !ok {
				return false, msg
			}
		}
	}
	return true, ""
}

func parseValidateAndExpandCiFile(file []byte) (ciJobs []CiJob, ok bool, notOkErrMsg string) {
	// Parse the file into the structs
	ciJobJsons, ok, notOkErrMsg := parseCiJobJsons(file)
	if !ok {
		return
	}
	ok, notOkErrMsg = isValidCiJobJsons("", ciJobJsons)
	if !ok {
		return
	}
	// Resolve templates, timeouts, etc
	ok, notOkErrMsg = resolveJobPayloadTemplates(ciJobJsons, 0)
	if !ok {
		return
	}
	resolveTimeouts(ciJobJsons)
	// Convert to []CiJob and validate
	ciJobs = convertToCiJob(ciJobJsons)
	ok, notOkErrMsg = isValidCiJobs(ciJobs)
	if !ok {
		return
	}
	return
}

func parseCiJobJsons(file []byte) ([]CiJobJson, bool, string) {
	// Try array form
	dec := json.NewDecoder(bytes.NewReader(file))
	dec.DisallowUnknownFields()
	var jsons []CiJobJson
	arrayErr := decodeWithBetterError(dec, &jsons)
	if arrayErr == nil {
		if dec.More() {
			return nil, false, "trailing data after array"
		}
		return jsons, true, ""
	}
	// Try single object form
	dec = json.NewDecoder(bytes.NewReader(file))
	dec.DisallowUnknownFields()
	var single CiJobJson
	singleErr := decodeWithBetterError(dec, &single)
	if singleErr == nil {
		if dec.More() {
			return nil, false, "trailing data after object"
		}
		return []CiJobJson{single}, true, ""
	}
	return nil, false, pickBetterError(file, arrayErr, singleErr).Error()
}

func resolveTimeouts(ciJobJson []CiJobJson) {
	for i := range ciJobJson {
		if ciJobJson[i].TimeoutMilliSeconds == 0 {
			if ciJobJson[i].TimeoutSeconds != 0 && ciJobJson[i].TimeoutMinutes != 0 {
				panic("should already have been validated")
			}
			if ciJobJson[i].TimeoutSeconds != 0 {
				ciJobJson[i].TimeoutMilliSeconds = ciJobJson[i].TimeoutSeconds * 1000
			}
			if ciJobJson[i].TimeoutMinutes != 0 {
				ciJobJson[i].TimeoutMilliSeconds = ciJobJson[i].TimeoutMinutes * 60 * 1000
			}
		}
	}
}

// All "resolve" functions should have been called at this point and we simply
// "downgrade" the extended payloads to the lower level one
func convertToCiJob(ciJobJsons []CiJobJson) (ciJobs []CiJob) {
	ciJobs = make([]CiJob, 0, len(ciJobJsons))
	for _, ciJobJson := range ciJobJsons {
		ciJob := CiJob{
			Job: JobPayload{
				Name:                ciJobJson.Name,
				ImageName:           ciJobJson.ImageName,
				TimeoutMilliSeconds: ciJobJson.TimeoutMilliSeconds,
			},
		}
		if len(ciJobJson.On) > 0 {
			ciJob.On = make([]JobTrigger, len(ciJobJson.On))
			copy(ciJob.On, ciJobJson.On)
		}
		if len(ciJobJson.Steps) > 0 {
			ciJob.Job.Steps = make([]JobStep, 0, len(ciJobJson.Steps))
			for _, st := range ciJobJson.Steps {
				ciJob.Job.Steps = append(ciJob.Job.Steps, JobStep{
					Run: st.Run,
					Dir: st.Dir,
				})
				if len(st.Env) != 0 {
					ciJob.Job.Steps[len(ciJob.Job.Steps)-1].Env = st.Env
				}
				if len(st.Secrets) != 0 {
					ciJob.Job.Steps[len(ciJob.Job.Steps)-1].Secrets = st.Secrets
				}
			}
		}
		ciJobs = append(ciJobs, ciJob)
	}
	return
}

const (
	maxJobs    = 1_000
	maxNameLen = 200
)

// Should only validate what's not validated by isValidPayloads, i.e. what's
// specifid about the extended payload such as Templates
func isValidCiJobJsons(prefix string, ciJobJsons []CiJobJson) (ok bool, notOkErrMsg string) {
	if len(ciJobJsons) == 0 {
		return false, "jobs can't be empty"
	}
	// Validate On
	for i := range ciJobJsons {
		msgPrefix := fmt.Sprintf("%sci_jobs[%d]", prefix, i)
		if len(ciJobJsons[i].On) != 0 {
			for triggerI, trigger := range ciJobJsons[i].On {
				if !slices.Contains(SupportedCiJobTriggers, trigger) {
					return false, fmt.Sprintf(
						"%s.On[%d]: bad value %q (must be one of %v)",
						msgPrefix, triggerI, trigger, SupportedCiJobTriggers)
				}
			}
		}
	}
	// Validate TemplateName
	for i := range ciJobJsons {
		msgPrefix := fmt.Sprintf("%sci_jobs[%d]", prefix, i)
		for stepI, step := range ciJobJsons[i].Steps {
			if step.TemplateName != "" {
				if len(step.Run) != 0 {
					return false, fmt.Sprintf(
						"%s.Steps[%d].Run: bad value %v (must be empty when TemplateName is used)",
						msgPrefix, stepI, step.Run)
				}
				if len(step.Env) != 0 {
					return false, fmt.Sprintf(
						"%s.Steps[%d].Env: bad value %v (must be empty when TemplateName is used)",
						msgPrefix, stepI, step.Env)
				}
				if len(step.Secrets) != 0 {
					return false, fmt.Sprintf(
						"%s.Steps[%d].Secrets: bad value %v (must be empty when TemplateName is used)",
						msgPrefix, stepI, step.Secrets)
				}
				if step.Dir != "" {
					return false, fmt.Sprintf(
						"%s.Steps[%d].Dir: bad value %v (must be empty when TemplateName is used)",
						msgPrefix, stepI, step.Dir)
				}
			}
		}
	}
	// Validate timeout in other units
	for i, ciJobJson := range ciJobJsons {
		msgPrefix := fmt.Sprintf("%sci_jobs[%d]", prefix, i)
		if ciJobJson.TimeoutSeconds < 0 {
			notOkErrMsg = fmt.Sprintf("%s.TimeoutSeconds: bad value %d < 0",
				msgPrefix, ciJobJson.TimeoutMilliSeconds)
			return
		}
		if ciJobJson.TimeoutMinutes < 0 {
			notOkErrMsg = fmt.Sprintf("%s.TimeoutMinutes: bad value %d < 0",
				msgPrefix, ciJobJson.TimeoutMilliSeconds)
			return
		}
		timeUnitCount := 0
		if ciJobJson.TimeoutMilliSeconds != 0 {
			timeUnitCount += 1
		}
		if ciJobJson.TimeoutSeconds != 0 {
			timeUnitCount += 1
		}
		if ciJobJson.TimeoutMinutes != 0 {
			timeUnitCount += 1
		}
		if timeUnitCount == 0 {
			ok = false
			notOkErrMsg = fmt.Sprintf(
				"%s.Timeout: TimeoutMilliSeconds, TimeoutSeconds or TimeoutMinutes must be > 0",
				msgPrefix)
			return
		}
		if timeUnitCount > 1 {
			ok = false
			notOkErrMsg = fmt.Sprintf(
				"%s.Timeout: only one of (TimeoutMilliSeconds, TimeoutSeconds, TimeoutMinutes) can be > 0",
				msgPrefix)
			return
		}
	}
	return true, ""
}

func isValidCiJobs(ciJobs []CiJob) (bool, string) {
	if len(ciJobs) == 0 {
		return false, "jobs can't be empty"
	}
	if len(ciJobs) > maxJobs {
		return false, fmt.Sprintf("too many jobs: %d exceeds maximum of %d", len(ciJobs), maxJobs)
	}
	// Names can't appear twice
	nameToId := make(map[string]int)
	for i, pl := range ciJobs {
		usedBy, ok := nameToId[pl.Job.Name]
		if ok {
			return false, fmt.Sprintf("jobs[%d].Name: %q already used by jobs[%d]", i, pl.Job.Name, usedBy)
		}
		nameToId[pl.Job.Name] = i
	}
	for i, ciJob := range ciJobs {
		if len(ciJob.On) > MaxCiJobOn {
			return false, fmt.Sprintf("jobs[%d].On: too many values (max %d)", i, MaxCiJobOn)
		}
		ok, notOkMsg := isValidJobPayload(fmt.Sprintf("jobs[%d]", i), ciJob.Job)
		if !ok {
			return false, notOkMsg
		}
	}
	return true, ""
}

func isValidJobPayload(errMsgJobName string, payload JobPayload) (bool, string) {
	if !slices.Contains(SupportedImages, payload.ImageName) {
		return false, fmt.Sprintf(
			"%s.ImageName: bad value %q (must be one of %v)",
			errMsgJobName, payload.ImageName, SupportedImages)
	}
	if payload.Name == "" {
		return false, fmt.Sprintf("%s.Name: can't be empty", errMsgJobName)
	}
	if len(payload.Name) > maxNameLen {
		return false, fmt.Sprintf(
			"%s.Name: bad length %d > %d", errMsgJobName, len(payload.Name), maxNameLen)
	}
	if payload.TimeoutMilliSeconds <= 0 {
		return false, fmt.Sprintf("%s.TimeoutMilliSeconds: bad value %d <= 0",
			errMsgJobName, payload.TimeoutMilliSeconds)
	}
	if len(payload.Steps) > MaxJobPayloadSteps {
		return false, fmt.Sprintf(
			"%s.Steps: too many steps (max %d)", errMsgJobName, MaxJobPayloadSteps)
	}
	for i := range payload.Steps {
		if len(payload.Steps[i].Env) > MaxJobPayloadEnv {
			return false, fmt.Sprintf(
				"%s.Steps[%d].Env: too many env vars (max %d)", errMsgJobName, i, MaxJobPayloadEnv)
		}
		if len(payload.Steps[i].Secrets) > MaxJobPayloadSecrets {
			return false, fmt.Sprintf(
				"%s.Steps[%d].Env: too many secrets (max %d)", errMsgJobName, i, MaxJobPayloadSecrets)
		}
	}
	return true, ""
}

const maxTemplateRecursion = 100

func resolveJobPayloadTemplates(jp []CiJobJson, currentRecursionCount int) (ok bool, notOkErrMsg string) {
	if currentRecursionCount > maxTemplateRecursion {
		ok = false
		notOkErrMsg = fmt.Sprintf("exceeded max template recursion %d", maxTemplateRecursion)
		return
	}

	mustRecurse := false
	for j := 0; j < len(jp); j++ {
		resolvedSteps := make([]CiJobStepJson, 0, len(jp[j].Steps))
		for s := 0; s < len(jp[j].Steps); s++ {
			if jp[j].Steps[s].TemplateName == "" {
				resolvedSteps = append(resolvedSteps, jp[j].Steps[s])
				continue
			}
			var steps []CiJobStepJson
			steps, ok = resolveJobStepTemplate(jp[j].Steps[s].TemplateName)
			if !ok {
				notOkErrMsg = fmt.Sprintf("unknown template %s", jp[j].Steps[s].TemplateName)
				return
			}
			for _, resolvedStep := range steps {
				if resolvedStep.TemplateName != "" {
					mustRecurse = true
				}
			}
			resolvedSteps = append(resolvedSteps, steps...)
		}
		jp[j].Steps = resolvedSteps
	}

	if mustRecurse {
		ok, notOkErrMsg = resolveJobPayloadTemplates(jp, currentRecursionCount+1)
		return
	}
	ok = true
	notOkErrMsg = ""
	return
}

func resolveJobStepTemplate(name JobStepTemplate) (s []CiJobStepJson, ok bool) {
	switch name {
	case GetCodeJobStepTemplate:
		s = []CiJobStepJson{
			{Dir: ".", Run: "tw init"},
			{Dir: ".", Run: "tw key $TWIGG_TOKEN"},
			{Dir: ".", Run: "tw server $REPO_ID"},
			{Dir: ".", Run: "tw pull $COMMIT_ID"},
		}
		ok = true
		return
	case DebugGetCodeJobStepTemplate:
		s = []CiJobStepJson{
			{Dir: ".", Run: "tw init --debug"},
			{Dir: ".", Run: "tw key $TWIGG_TOKEN --debug"},
			{Dir: ".", Run: "tw server $REPO_ID --debug"},
			{Dir: ".", Run: "tw pull $COMMIT_ID --debug"},
		}
		ok = true
		return
	default:
		ok = false
		return
	}
}

// decode to json but shows a more readable error
func decodeWithBetterError(dec *json.Decoder, v any) error {
	if err := dec.Decode(v); err != nil {
		switch e := err.(type) {
		case *json.SyntaxError:
			return fmt.Errorf("invalid JSON syntax at byte %d", e.Offset)

		case *json.UnmarshalTypeError:
			if e.Field != "" {
				return fmt.Errorf(
					"invalid value for field '%s': expected %s but got %s",
					e.Field, e.Type, e.Value,
				)
			}
			return fmt.Errorf(
				"invalid value: expected %s but got %s (byte %d)",
				e.Type, e.Value, e.Offset,
			)
		case *json.InvalidUnmarshalError:
			panic("called decodeWithBetterError with invalid target")

		default:
			msg := err.Error()
			if strings.HasPrefix(msg, "json: unknown field ") {
				field := strings.TrimPrefix(msg, "json: unknown field ")
				return fmt.Errorf("unknown field %s", field)
			}
			if errors.Is(err, io.EOF) {
				return fmt.Errorf("empty JSON input")
			}
			return fmt.Errorf("invalid JSON format")
		}
	}
	return nil
}

func pickBetterError(input []byte, arrErr, objErr error) error {
	trim := bytes.TrimSpace(input)
	if len(trim) == 0 {
		return fmt.Errorf("empty json")
	}
	switch trim[0] {
	case '[':
		return arrErr
	case '{':
		return objErr
	default:
		// fallback: prefer more specific error
		if isTypeError(objErr) {
			return objErr
		}
		return arrErr
	}
}
func isTypeError(err error) bool {
	_, ok := err.(*json.UnmarshalTypeError)
	return ok
}