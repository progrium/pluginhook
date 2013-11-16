package main

import (
	"os"
	"fmt"
	"os/exec"
	"syscall"
	"path/filepath"
	"log"
	"flag"
	"os/signal"
	"code.google.com/p/go.crypto/ssh/terminal"
)

func main() {
	var singleHook = flag.Bool("s", false, "Only run a single hook")
	flag.Parse()

	pluginPath := os.Getenv("PLUGIN_PATH")
	if pluginPath == "" {
		log.Fatal("[ERROR] Unable to locate plugins: set $PLUGIN_PATH\n")
		os.Exit(1)
	}
	if flag.NArg() < 1 {
		log.Fatal("[ERROR] Hook name argument is required\n")
		os.Exit(1)
	}

	// Ignore signals. They are sent to the child process anyway. Just wait for the hooks to stop
	sigc := make(chan os.Signal)
	signal.Notify(sigc, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	var matches, _ = filepath.Glob(fmt.Sprintf("%s/*/%s", pluginPath, flag.Arg(0)))

	if *singleHook {
		if (len(matches) == 0) {
			log.Fatal("[ERROR] No hook with the specified name exists\n")
			os.Exit(1)
		}

		cmd := exec.Command(matches[0], flag.Args()[1:]...)
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		cmd.Stdin = os.Stdin

		err := cmd.Run()
		if msg, ok := err.(*exec.ExitError); ok { // there is error code
			os.Exit(msg.Sys().(syscall.WaitStatus).ExitStatus())
		}
	} else {
		cmds := make([]exec.Cmd, 0)
		for _, hook := range matches {
			cmd := exec.Command(hook, flag.Args()[1:]...)
			cmds = append(cmds, *cmd)
		}
		done := make(chan bool, len(cmds))
		for i := len(cmds) - 1; i >= 0; i-- {
			cmds[i].Stderr = os.Stderr

			if i == len(cmds)-1 {
				cmds[i].Stdout = os.Stdout
			}
			if i > 0 {
				stdout, err := cmds[i-1].StdoutPipe()
				if err != nil {
					log.Fatal(err)
				}
				cmds[i].Stdin = stdout
			}
			if i == 0 && !terminal.IsTerminal(syscall.Stdin) {
				cmds[i].Stdin = os.Stdin
			}
			go func(cmd exec.Cmd) {
				err := cmd.Run()
				if msg, ok := err.(*exec.ExitError); ok { // there is error code
					os.Exit(msg.Sys().(syscall.WaitStatus).ExitStatus())
				}
				done <- true
			}(cmds[i])
		}
		for i := 0; i < len(cmds); i++ {
			<-done
		}
	}
}
