package runners

import (
	"errors"
	"os/exec"
)

func checkDockerEnv() (bool, error) {
	if _, err := exec.LookPath("docker"); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	for _, img := range []string{dockerBaseRunnerImage, dockerGoRunnerImage, dockerBunRunnerImage} {
		_, err := exec.Command("docker", "image", "inspect", img).CombinedOutput()
		if err == nil {
			continue
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) || errors.Is(err, exec.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func checkLxdEnv() (bool, error) {
	if _, err := exec.LookPath("lxc"); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	_, err := exec.Command("lxc", "image", "info", lxdVMImage).CombinedOutput()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
