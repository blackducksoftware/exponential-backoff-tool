/*Package cmd Root command for Exponential Backoff

Copyright © 2020 Robert Piskule piskule@synopsys.com

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/Knetic/govaluate"
	logging "github.com/op/go-logging"
	"github.com/spf13/cobra"
	ini "gopkg.in/ini.v1"
)

var log *logging.Logger

// We load the flags from INI files as well as from the command line...
// So the command line values by default are unset, represented as -1
// However, the command line values also take precedence...
// If the value remains unset throughout the loading process, these
// will be the default values (not -1)
var _realRetryDefault = 0
var _realMaxDurationDefault = 0
var _realExpressionDefault = "0"

// The parameters this command takes in
var _debug bool
var _verbose bool
var _expression string
var _retries int
var _duration int
var _iniFile string
var _retryOnExitCodes string
var _retryOnStringMatches string

// The command definition
var rootCmd = &cobra.Command{
	Use:   "eb",
	Short: "Exponential Backoff Tool",
	Long: `Exponential Backoff Tool

This tool is used to wrap unreliable tools such as git
gcloud, awscli, vault, and any other tool requiring an
API to connect to, and build in configurable exponential
backoff based off of either error messages, or exit codes.`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return errors.New("Requires a command to run")
		}
		return nil
	},

	Run: func(cmd *cobra.Command, args []string) {
		os.Exit(ExponentialBackoff(args, _debug, _verbose, _expression, _retries, _duration, _iniFile, _retryOnExitCodes, _retryOnStringMatches))
	},
}

func getStringParameter(cfg *ini.File, command string, key string, currentValue string, nullValue string) string {
	log.Debug("Searching for ", command, "/", key, "...")
	if cfg.Section(command).HasKey(key) {
		value := cfg.Section(command).Key(key).String()
		if currentValue == nullValue {
			log.Debug("Found", value)
			return value
		}
	}
	return currentValue
}

func getIntParameter(cfg *ini.File, command string, key string, currentValue int, nullValue int) int {
	if cfg.Section(command).HasKey(key) {
		value, _ := cfg.Section(command).Key(key).Int()
		if currentValue == nullValue {
			return value
		}
	}
	return currentValue
}

// ExponentialBackoff this is a separate function because perhaps somebody wants to run this
// without calling the command line in their golang code
func ExponentialBackoff(command []string, debug bool, verbose bool, expression string, retries int, duration int, iniFile string, retryOnExitCodes string, retryOnStringMatches string) int {

	// Configure the logging
	var format = logging.MustStringFormatter(
		`%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}`,
	)
	backend1 := logging.NewLogBackend(os.Stderr, "", 0)
	backend1Formatter := logging.NewBackendFormatter(backend1, format)
	backend1Leveled := logging.AddModuleLevel(backend1Formatter)

	backend1Leveled.SetLevel(logging.ERROR, "")
	if verbose {
		backend1Leveled.SetLevel(logging.INFO, "")
	}
	if debug {
		backend1Leveled.SetLevel(logging.DEBUG, "")
	}
	logging.SetBackend(backend1Leveled)
	log = logging.MustGetLogger("root")
	log.Debug("Command Parts")
	for _, element := range command {
		log.Debug(element)
	}

	// Treat the retry strings list as a row from a CSV file so we don't need to do intelligent parsing
	// The command we are running can be passed in as a string "kubectl get pods" or as multiple arguements ["kubectl","get","pods"].
	// The former is required when command line parameters for this command override the parameters of the sub-command.
	// The latter is provided for convenience (much like the `watch` command)
	if len(command) == 1 {
		log.Debug("Parsing:", command[0])
		r := csv.NewReader(strings.NewReader(command[0]))
		r.Comma = ' '
		var err error
		command, err = r.Read()
		if err != nil {
			log.Critical(err)
			return 1
		}
		log.Debug("Command string parsed as:\n")
		for _, field := range command {
			log.Debug(field)
		}
	}

	// Configure the default location of the INI file
	loadIniFile := iniFile
	if iniFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Warning("Fail to read file: ", err)
			log.Error("Failed to find home directory!")
			loadIniFile = "eb.ini"
		} else {
			loadIniFile = home + string(os.PathSeparator) + "eb.ini"
		}
		log.Info("No INI file specified. Using " + loadIniFile + " if it exists.")
	}

	// Read the INI file - Local values override global values
	cfg, err := ini.Load(loadIniFile)
	if err != nil {
		if iniFile != "" {
			log.Critical("Fail to read file: ", err)
			return 1
		}
		log.Warning("Fail to read file: ", err)
	} else {
		log.Info("Loaded INI file:", loadIniFile)
		log.Info("Loading configuration settings for:", command[0])

		// By default, command line parameters come first...

		// If those are not defined,  look in the local section
		retryOnExitCodes = getStringParameter(cfg, command[0], "retry_on_exit_codes", retryOnExitCodes, "")
		retryOnStringMatches = getStringParameter(cfg, command[0], "retry_on_string_matches", retryOnStringMatches, "")
		expression = getStringParameter(cfg, command[0], "expression", expression, "")
		retries = getIntParameter(cfg, command[0], "retries", retries, -1)
		duration = getIntParameter(cfg, command[0], "duration", duration, -1)

		log.Debug("After Loading Local INI Settings:")
		log.Debug("Expression: ", expression)
		log.Debug("Retries: ", retries)
		log.Debug("Duration: ", duration)
		log.Debug("Retry On Exit Codes: ", retryOnExitCodes)
		log.Debug("Retry On String Matches: ", retryOnStringMatches)

		// If  not defined there, check the global section
		expression = getStringParameter(cfg, "", "expression", expression, "")
		retries = getIntParameter(cfg, "", "retries", retries, -1)
		duration = getIntParameter(cfg, "", "duration", duration, -1)

		log.Debug("After Loading Global INI Settings:")
		log.Debug("Expression: ", expression)
		log.Debug("Retries: ", retries)
		log.Debug("Duration: ", duration)
		log.Debug("Retry On Exit Codes: ", retryOnExitCodes)
		log.Debug("Retry On String Matches: ", retryOnStringMatches)
	}

	// If after checking everywhere, the values are still -1, set them to their defaults
	if expression == "" {
		expression = _realExpressionDefault
	}

	if retries == -1 && duration == -1 {
		retries = _realRetryDefault
		duration = _realMaxDurationDefault
	} /* else if retries == -1 && duration != -1 {
		retries = 65535
	} else if retries != -1 && duration == -1 {
		duration = 65535
	}*/

	// Treat the retry codes list as a row from a CSV file so we don't need to do intelligent parsing
	var ignoreExitCodes []int
	if retryOnExitCodes != "" {
		r := csv.NewReader(strings.NewReader(retryOnExitCodes))
		ignoreExitCodesStr, err := r.Read()
		if err != nil {
			log.Critical(err)
			return 1
		}

		log.Debug("Retrying on the following exit codes:\n")
		for _, field := range ignoreExitCodesStr {
			code, err := strconv.Atoi(field)
			if err != nil {
				log.Critical(err)
				return 1
			}
			ignoreExitCodes = append(ignoreExitCodes, code)
			log.Debug(code)
		}
	}

	// Treat the retry strings list as a row from a CSV file so we don't need to do intelligent parsing
	var ignoreStrings []string
	if retryOnStringMatches != "" {
		r := csv.NewReader(strings.NewReader(retryOnStringMatches))
		ignoreStringsStr, err := r.Read()
		if err != nil {
			log.Critical(err)
			return 1
		}
		log.Debug("Retrying if the following strings are found:\n")
		for _, field := range ignoreStringsStr {
			log.Debug(field)
			ignoreStrings = append(ignoreStrings, field)
		}
	}

	log.Info("------ Settings ------")
	log.Info("Expression             : ", expression)
	log.Info("Retries                : ", retries)
	log.Info("Duration               : ", duration)
	log.Info("Retry On Exit Codes    : ", retryOnExitCodes)
	log.Info("Retry On String Matches: ", retryOnStringMatches)
	log.Info("Command to Run         : ", command)
	log.Info("----------------------")

	xIncrement := 0
	start := time.Now()
	for {
		log.Debug("Running ", command[0])
		cmd := exec.Command(command[0], command[1:len(command)]...)
		var out bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &stderr
		cmd.Run()
		exitCode := cmd.ProcessState.ExitCode()
		log.Debug("Command exitted with ", exitCode)
		needToExit := true

		// Do not exit if the exit code matched our input retryOnExitCodes
		if exitCode != 0 {
			for i := range ignoreExitCodes {
				if exitCode == ignoreExitCodes[i] {
					log.Debug("Program exited with code: ", exitCode, ". Restarting.")
					needToExit = false
				}
			}
		}

		// Do not exit if output / stderr from the command contained a string in our retryOnMatchedStrings list
		for i := range ignoreStrings {
			if strings.Contains(out.String(), ignoreStrings[i]) {
				log.Debug("Output stream contained: ", ignoreStrings[i], ". Restarting.")
				needToExit = false
			}
			if strings.Contains(stderr.String(), ignoreStrings[i]) {
				log.Debug("Error stream contained: ", ignoreStrings[i], ". Restarting.")
				needToExit = false
			}
		}

		if needToExit {
			log.Debug("Exiting with ", exitCode)
			os.Stderr.WriteString(stderr.String())
			fmt.Print(out.String())
			return exitCode
		}

		log.Info(stderr.String())
		log.Info(out.String())

		xIncrement++
		t := time.Now()
		elapsed := t.Sub(start)

		if xIncrement > retries && retries != -1 {
			log.Warning("Failed to complete command due to retries exhausted:", command)
			log.Warning("Exitting with error code:", exitCode)
			os.Stderr.WriteString(stderr.String())
			fmt.Print(out.String())
			return exitCode
		}
		if elapsed > time.Duration(duration)*time.Second && duration != -1 {
			log.Warning("Failed to complete command due to maximum runtime exhausted:", command)
			log.Warning("Exitting with error code:", exitCode)
			os.Stderr.WriteString(stderr.String())
			fmt.Print(out.String())
			return exitCode
		}

		expression, err := govaluate.NewEvaluableExpression(expression)
		if err != nil {
			log.Error("Formula cannot be evaluated!")
			return 2
		}

		s1 := rand.NewSource(time.Now().UnixNano())
		r1 := rand.New(s1)
		parameters := make(map[string]interface{}, 8)
		parameters["x"] = xIncrement - 1
		parameters["i"] = xIncrement
		parameters["r"] = r1.Float64()

		log.Debug("Expression:", expression, "x:", xIncrement-1)
		result, err := expression.Evaluate(parameters)
		//fmt.Printf("(%v, %T)\n", result, result)

		if err != nil {
			log.Error("Formula failed to be evaluate!")
			return 3
		}
		value, _ := result.(float64)

		log.Debug("Formula calculation:", value)

		log.Info("Program exitted with exit code: ", cmd.ProcessState.ExitCode())
		sleepForD := time.Duration(value) * time.Second
		log.Info("Time to sleeping for before retrying: ", result)

		time.Sleep(sleepForD)
	}
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	// eb is a root command with no sub-commands, so everything is global and persistant
	// use hyphens instead of camelCase because that is what curl does
	rootCmd.PersistentFlags().StringVarP(&_iniFile, "ini-file", "f", "", "An INI file to load with tool settings (Default: $HOME/.eb.ini)\nThe INI file supports global and local parameters.\nLocal parameters override global parameters.\nSample ini")
	rootCmd.PersistentFlags().StringVarP(&_expression, "expression", "e", "", "A mathmematical expression representing the time to wait on each retry (Default: \"0\").\nThe variable 'x' is the current iteration (0 based).\nThe variable 'i' is the current iteration (1 based).\nThe variable 'r' is a random float from 0-1.\nExamples: \"x*15+15\", \"x*x\", \"(x*x)+(10*r)\"\n")
	rootCmd.PersistentFlags().IntVarP(&_retries, "retries", "r", -1, "The number of times to retry the command (Default: 0)")
	rootCmd.PersistentFlags().IntVarP(&_duration, "duration", "t", -1, "How long to keep retrying for (Default: 0)")
	rootCmd.PersistentFlags().StringVarP(&_retryOnExitCodes, "retry-on-exit-codes", "c", "", "A comma delimited list of exit codes to try on.")
	rootCmd.PersistentFlags().StringVarP(&_retryOnStringMatches, "retry-on-string-matches", "s", "", "A comma delimited list of strings found in stderr or stdout to retry on.")
	rootCmd.PersistentFlags().BoolVarP(&_verbose, "verbose", "v", false, "Enable Verbose Output")
	rootCmd.PersistentFlags().BoolVarP(&_debug, "debug", "d", false, "Enable Debugging")
}
