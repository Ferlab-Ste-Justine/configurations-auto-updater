package cmd

import (
	"fmt"
	"os/exec"
)

func ExecCommand(command []string, retries uint64) error {
	out, err := exec.Command(command[0], command[1:]...).Output()

	if err != nil {
		if retries > 0 {
			fmt.Println("Warning: Following error occured on post-updated command, will retry: ", err.Error())
			return ExecCommand(command, retries - 1)
		}

		return err
	}

	if len(out) > 0 {
		fmt.Print(string(out))
	}
	
	return nil
}