package runnerlib

import (
	"fmt"
	"io"
	"monorepo/twigg-runner/tse"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type runner struct {
	absPathToWorkdir string
	stdOut           io.Writer
	stdErr           io.Writer
}

func (r runner) Run(job JobPayload) (exitCode int) {
	for stepI, step := range job.Steps {
		if len(step.Run) == 0 {
			_, _ = r.stdErr.Write([]byte(fmt.Sprintf("Steps[%d].Run is empty\n", stepI)))
			exitCode = 1
			return
		}
		if step.Env == nil {
			step.Env = make(map[string]string)
		}

		secretsVals, ok := r.getSecretsVals(job, stepI, &step, &exitCode)
		if !ok {
			return
		}

		scrubberOut, closeScrubberOut, err := tse.NewTseScrubber(r.stdOut, secretsVals)
		if err != nil {
			r.stdErr.Write([]byte(
				fmt.Sprintf("failed to initialize stdout log sanitizer: %s\n", err)))
			exitCode = 1
			return
		}
		scrubberErr, closeScrubberErr, err := tse.NewTseScrubber(r.stdErr, secretsVals)
		if err != nil {
			r.stdErr.Write([]byte(
				fmt.Sprintf("failed to initialize stderr log sanitizer: %s\n", err)))
			_ = closeScrubberOut()
			exitCode = 1
			return
		}
		// https://no-color.org/ -> disable ASII escape codes
		step.Env["NO_COLOR"] = "true"
		shellArgs := []string{"-c", step.Run}
		cmd := exec.Command("sh", shellArgs...)
		cmd.Stdin = nil
		cmd.Stdout = &scrubberOut
		cmd.Stderr = &scrubberErr
		cmd.Dir = r.absPathToWorkdir
		if step.Dir != "" && step.Dir != "." {
			cmd.Dir = filepath.Join(r.absPathToWorkdir, step.Dir)
			err := os.MkdirAll(cmd.Dir, 0755)
			if err != nil {
				_, _ = scrubberErr.Write([]byte(fmt.Sprintf("mkdir %q failed: %s\n", cmd.Dir, err)))
				_ = closeScrubberErr()
				_ = closeScrubberOut()
				exitCode = 1
				return
			}
		}
		envVars := []string{}
		for key, val := range step.Env {
			envVars = append(envVars, fmt.Sprintf("%s=%s", key, val))
		}
		cmd.Env = append(os.Environ(), envVars...)
		start := time.Now()
		if step.Dir == "" || step.Dir == "." {
			scrubberOut.Write([]byte(fmt.Sprintf("Steps[%d]: run %q ...\n", stepI, step.Run)))
		} else {
			scrubberOut.Write([]byte(fmt.Sprintf("Steps[%d]: run %q in %s/ ...\n",
				stepI, step.Run, step.Dir)))
		}
		// ok to call bc up to this point all data written was controlled by us
		// and there is no risk of split secret. UnsafeFlush can only cause
		// leaks for partial secret writes:
		// Write("SE"), UnsafeFush(), Write("CRET"), UnsafeFush() -> leaks "SECRET"
		// This can't happen in this scenario
		_ = scrubberOut.UnsafeFlush()
		err = cmd.Run()
		if err != nil {
			gotExitCode := false
			if exitError, ok := err.(*exec.ExitError); ok {
				exitCode = exitError.ExitCode()
				gotExitCode = true
			} else {
				exitCode = 1
			}
			if gotExitCode {
				scrubberErr.Write([]byte(fmt.Sprintf(
					"Steps[%d] failed after %s with exit code %d\n",
					stepI, time.Since(start), exitCode)))
			} else {
				scrubberErr.Write([]byte(fmt.Sprintf("Steps[%d] failed after %s\n",
					stepI, time.Since(start))))
			}
			_ = closeScrubberOut()
			_ = closeScrubberErr()
			return
		}
		scrubberOut.Write([]byte(fmt.Sprintf("Steps[%d] successfully finished in %s\n", stepI, time.Since(start))))
		scrubberOutErr := closeScrubberOut()
		scrubberErrErr := closeScrubberErr()
		if scrubberOutErr != nil || scrubberErrErr != nil {
			exitCode = 1
			return
		}
	}
	exitCode = 0
	return
}

func (r runner) getSecretsVals(job JobPayload, stepI int, step *JobStep, exitCode *int) (secretsVals []string, ok bool) {
	nSecrets := len(step.Secrets)
	// We expect job.Token to match TwiggTokenEnvVar's value, but this function
	// will also work even if they don't. It'll consider that both can be
	// different and both are considered a secret.
	jobTokenIsSecret := job.Token != ""
	if jobTokenIsSecret {
		nSecrets += 1
	}
	envVarTokenIsSecret := false
	envVarToken, hasEnvVarToken := step.Env[TwiggTokenEnvVarName]
	envVarTokenIsSecret = hasEnvVarToken && envVarToken != "" && envVarToken != job.Token
	if envVarTokenIsSecret {
		nSecrets += 1
	}

	secretsVals = make([]string, 0, nSecrets)
	for _, secret := range step.Secrets {
		secretVal, contains := step.Env[secret]
		if !contains {
			r.stdErr.Write([]byte(fmt.Sprintf("Steps[%d] Secret %q not found\n", stepI, secret)))
			*exitCode = 1
			return
		}
		if secretVal == "" {
			r.stdErr.Write([]byte(fmt.Sprintf("Steps[%d] Secret %q is empty\n", stepI, secret)))
			*exitCode = 1
			return
		}
		secretsVals = append(secretsVals, secretVal)
	}
	if jobTokenIsSecret {
		secretsVals = append(secretsVals, job.Token)
	}
	if envVarTokenIsSecret {
		secretsVals = append(secretsVals, envVarToken)
	}
	ok = true
	return
}