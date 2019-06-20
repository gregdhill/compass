package core

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/monax/compass/core/schema"
	"github.com/monax/compass/util"
	log "github.com/sirupsen/logrus"
)

func checkRequires(values util.Values, reqs util.Values) error {
	for k, v := range reqs {
		if _, exists := values[k]; !exists {
			return fmt.Errorf("argument '%s' not given", k)
		} else if values[k] != v {
			return fmt.Errorf("argument '%s' not given value '%s'", k, v)
		}
	}
	return nil
}

// Create installs / upgrades resource
func Create(stg *schema.Stage, logger *log.Entry, key string, global util.Values, force bool) error {
	// stop if already installed and abandoned
	installed, _ := stg.Status()
	if installed && !force && stg.Forget {
		logger.Infof("Ignoring: %s", key)
		return nil
	}

	if err := checkRequires(global, stg.Requires); err != nil {
		logger.Infof("Ignoring: %s: %s", key, err.Error())
		return nil
	}

	shellVars := global.ToSlice()
	if err := shellTasks(stg.Jobs.Before, shellVars); err != nil {
		return err
	}

	if obj := stg.GetInput(); obj != nil {
		fmt.Println(string(obj))
	}

	logger.Infof("Installing: %s", key)
	if err := stg.InstallOrUpgrade(); err != nil {
		logger.Fatalf("Failed to install %s: %s", key, err)
		return err
	}
	logger.Infof("Installed: %s", key)

	if err := shellTasks(stg.Jobs.After, shellVars); err != nil {
		return err
	}

	return nil
}

// Destroy removes resource
func Destroy(stg *schema.Stage, logger *log.Entry, key string, global util.Values, force bool) error {
	// only continue if required variables are set
	if err := checkRequires(global, stg.Requires); err != nil {
		logger.Infof("Ignoring: %s: %s", key, err.Error())
		return nil
	}

	// don't delete by default
	if !force && stg.Forget {
		logger.Infof("Ignoring: %s", key)
		return fmt.Errorf("Not deleting stage %s", key)
	}

	logger.Infof("Deleting: %s", key)

	return stg.Delete()
}

func shellTasks(jobs []string, values []string) error {
	for _, command := range jobs {
		log.Infof("running job: %s\n", command)
		out, err := Shell(command, values)
		if out != nil {
			fmt.Println(string(out))
		}
		if err != nil {
			return fmt.Errorf("job '%s' exited with error: %v", command, err)
		}
	}
	return nil
}

// Shell runs any given command
func Shell(command string, values []string) ([]byte, error) {
	args := strings.Fields(command)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = append(values, os.Environ()...)
	return cmd.Output()
}
