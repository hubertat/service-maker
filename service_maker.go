package servicemaker

import (
	"bytes"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"text/template"
)

type ServiceMaker struct {
	User               string
	UserGroups         []string
	ServicePath        string
	ServiceDescription string
	ExecDir            string
	ExecName           string
	SkipConfig         bool
	ExampleConfig      string
}

func (sm *ServiceMaker) systemdServiceContent() (out []byte, err error) {
	tmpl, err := template.New("service").Parse(`[Unit]
	Description={{.ServiceDescription}}
	After=network.target
	StartLimitIntervalSec=0
	
	[Service]
	Type=simple
	Restart=always
	RestartSec=5
	User={{.User}}
	WorkingDirectory={{.ExecDir}}
	ExecStart={{.ExecDir}}/{{.ExecName}}
	
	[Install]
	WantedBy=multi-user.target`)

	if err != nil {
		return
	}
	var buff bytes.Buffer
	err = tmpl.Execute(&buff, sm)
	if err != nil {
		return
	}
	out = buff.Bytes()
	return
}

func (sm *ServiceMaker) linuxCheckAndUpdateUser() error {
	var outErr bytes.Buffer
	userCheckCommand := exec.Command("id", sm.User)
	if userCheckCommand.Run() != nil {
		userCreateCommand := exec.Command("useradd", "-r", "-s", "/bin/false", sm.User)
		userCreateCommand.Stderr = &outErr
		err := userCreateCommand.Run()
		if err != nil {
			return fmt.Errorf("error creating user %s:\n%v\n%s\n", sm.User, err, outErr.String())
		}
	}

	for _, groupName := range sm.UserGroups {
		groupAppendCommand := exec.Command("usermod", "-a", "-G", groupName, sm.User)
		groupAppendCommand.Stderr = &outErr
		err := groupAppendCommand.Run()
		if err != nil {
			return fmt.Errorf("error appending dialout group for user %s:\n%v\n%s\n", sm.User, err, outErr.String())
		}
	}

	return nil
}

func (sm *ServiceMaker) linuxCopyFiles() error {
	fsys := os.DirFS(sm.ExecDir)
	serviceDir, err := fs.Stat(fsys, ".")
	if err != nil {
		if os.IsNotExist(err) {
			err = os.Mkdir(sm.ExecDir, os.FileMode(0655))
			if err != nil {
				return fmt.Errorf("error creating directory (%s) for executable: \n%v", sm.ExecDir, err)
			}
			serviceDir, err = fs.Stat(fsys, ".")
			if err != nil {
				return fmt.Errorf("error getting directory: %v", err)
			}
		} else {
			return fmt.Errorf("error when opening exec dir %s: \n%v", sm.ExecDir, err)
		}
	}

	if !serviceDir.IsDir() {
		return fmt.Errorf("path %s is not Dir, but it needs to be!", serviceDir.Name())
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("Error checking executable path: \n%v", err)
	}

	copyCommand := exec.Command("cp", exePath, filepath.Join(sm.ExecDir, sm.ExecName))
	var stdErr bytes.Buffer
	copyCommand.Stderr = &stdErr
	err = copyCommand.Run()
	if err != nil {
		return fmt.Errorf("error copying excecutable:\n%v\n%s\n", err, stdErr.String())
	}

	if !sm.SkipConfig {
		_, err = fs.Stat(fsys, "config.json")
		if err != nil {
			if os.IsNotExist(err) {
				localFs := os.DirFS(".")
				localConfig, err := fs.ReadFile(localFs, "config.json")
				if err == nil {
					err = ioutil.WriteFile(filepath.Join(sm.ExecDir, "config.json"), localConfig, os.FileMode(0644))
				} else {
					err = ioutil.WriteFile(filepath.Join(sm.ExecDir, "config.json"), []byte(sm.ExampleConfig), os.FileMode(0644))
				}
				if err != nil {
					return fmt.Errorf("error writing config.json to %s:\n%v\n", sm.ExecDir, err)
				}

			}
		}
	}

	ownCommand := exec.Command("chown", "-R", sm.User, sm.ExecDir)
	ownCommand.Stderr = &stdErr
	err = ownCommand.Run()
	if err != nil {
		return fmt.Errorf("error running chown on %s:\n%v\n%s\n", sm.ExecDir, err, stdErr.String())
	}

	return nil
}

func (sm *ServiceMaker) linuxInstallService() error {

	if !(isSystemd()) {
		return fmt.Errorf("Systemd not found but required, check if installed.")
	}

	err := sm.linuxCheckAndUpdateUser()
	if err != nil {
		return err
	}

	err = sm.linuxCopyFiles()
	if err != nil {
		return err
	}
	configInBytes, err := sm.systemdServiceContent()
	if err != nil {
		return fmt.Errorf("error parsing content of service file: %v", err)
	}
	err = ioutil.WriteFile(sm.ServicePath, configInBytes, os.FileMode(0644))
	if err != nil {
		return fmt.Errorf("error creating file %s: 'n%v", sm.ServicePath, err)
	}

	enableCommand := exec.Command("systemctl", "enable", filepath.Base(sm.ServicePath))
	var outErr bytes.Buffer
	enableCommand.Stderr = &outErr
	err = enableCommand.Run()
	if err != nil {
		return fmt.Errorf("error enabling systemd service %s:\n%v\n%s\n", sm.ServicePath, err, outErr.String())
	}

	return nil
}

func (sm *ServiceMaker) InstallService() error {
	switch runtime.GOOS {
	case "linux":
		return sm.linuxInstallService()
	default:
		return fmt.Errorf("Error ServiceMaker InstallService failed: unsupported operating system.")
	}
}

func isSystemd() bool {
	if _, err := os.Stat("/run/systemd/system"); err == nil {
		return true
	}
	return false
}
