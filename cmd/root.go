/*
* Copyright 2020-present, Synopsys, Inc. * All rights reserved.
*
* This source code is licensed under the Apache-2.0 license found in
* the LICENSE file in the root directory of this source tree. */
package cmd

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Knetic/govaluate"
	shellwords "github.com/mattn/go-shellwords"
	logging "github.com/op/go-logging"
	"github.com/spf13/cobra"
	ini "gopkg.in/ini.v1"
)

var log *logging.Logger

var _releaseVersion = "0.0.4"

// The parameters this command takes in
var _debug bool
var _kill bool
var _verbose bool
var _version bool
var _expression string
var _retries int
var _duration int
var _iniFile string
var _printRetryOnFailure bool
var _printVerboseRetryOnFailure bool
var _retryOnAll bool
var _retryOnExitCodes string
var _retryOnStringMatches string
var _retryOnRegexpMatches string
var _successOnExitCodes string
var _successOnStringMatches string
var _successOnRegexpMatches string
var _failOnStringMatches string
var _failOnRegexpMatches string
var _failUnlessStringMatches string
var _failUnlessRegexpMatches string
var _performOnFailure string

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
		return nil
	},

	Run: func(cmd *cobra.Command, args []string) {
		configureLogging(_verbose, _debug)
		if _version {
			fmt.Println(_releaseVersion)
			os.Exit(1)
		}
		if _kill {
			s1 := rand.NewSource(time.Now().UnixNano())
			r1 := rand.New(s1)
			if r1.Float64() > .875 {
				fmt.Println("Sample Error: TLS Timeout")
				os.Exit(3)
			} else if r1.Float64() > .75 {
				fmt.Println("Sample Error: Request Timed Out")
				os.Exit(3)
			} else if r1.Float64() > .675 {
				// Yes, this actually happens in the real world (see S3 documentation)
				fmt.Println("Sample Error: 200 OK: The web request failed")
				// And yes, it will return a success code
				os.Exit(0)
			} else if r1.Float64() > .50 {
				fmt.Println("Sample Error: Rate Limit Exceeded")
				os.Exit(2)
			} else if r1.Float64() > .375 {
				fmt.Println("Sample Error: Already Exists")
				os.Exit(1)
			} else if r1.Float64() > .25 {
				fmt.Println("Sample Error: Concurrent Modification")
				os.Exit(1)
			} else {
				fmt.Println("Sample Success: 200 OK: Your request succeeded")
				os.Exit(0)
			}
		}
		if len(args) < 1 {
			cmd.Help()
			os.Stderr.WriteString("\nExponential Backoff Tool requires at least 1 argument!\n")
			os.Exit(1)
		}
		command := convertArgs(args)
		expression, retries, duration, retryOnAll, ignoreExitCodes, ignoreStrings, ignoreRegexps, successOnExitCodes, successOnStrings, successOnRegexps, performOnFailure, failOnStrings, failOnRegexps, failUnlessStrings, failUnlessRegexps, printRetryOnFailure, printVerboseRetryOnFailure  := loadParameters(cmd, command[0], _iniFile, _expression, _retries, _duration, _retryOnAll, _retryOnExitCodes, _retryOnStringMatches, _retryOnRegexpMatches, _successOnExitCodes, _successOnStringMatches, _successOnRegexpMatches, _performOnFailure, _failOnStringMatches, _failOnRegexpMatches, _failUnlessStringMatches, _failUnlessRegexpMatches, _printRetryOnFailure, _printVerboseRetryOnFailure)
		os.Exit(ExponentialBackoff(command, expression, retries, duration, retryOnAll, ignoreExitCodes, ignoreStrings, ignoreRegexps, successOnExitCodes, successOnStrings, successOnRegexps, performOnFailure, failOnStrings, failOnRegexps, failUnlessStrings, failUnlessRegexps, printRetryOnFailure, printVerboseRetryOnFailure))
	},
}

func convertArgs(command []string) []string {
	log.Debug("Command Parts")
	for _, element := range command {
		log.Debug(element)
	}

	if len(command) == 1 {
		commandString := command[0]
		log.Debug("Parsing:", commandString)
		var err error
		command, err = shellwords.Parse(commandString)
		if err != nil {
			log.Error("Unable to parse command input:", commandString)
			log.Critical(err)
			os.Exit(1)
		}

		log.Debug("Command string parsed as:\n")
		for _, field := range command {
			log.Debug(field)
		}
	}
	return command
}

func configureLogging(verbose bool, debug bool) {
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
}

func getStringParameter(cmd *cobra.Command, cfg *ini.File, command string, key string, currentValue string, flag string) string {
	log.Debug("Searching for ", command, "/", key, "...")
	if cfg.Section(command).HasKey(key) {
		value := cfg.Section(command).Key(key).String()
		if !cmd.Flags().Changed(flag) {
			log.Debug("Found", value)
			return value
		}
	}
	return currentValue
}

func getIntParameter(cmd *cobra.Command, cfg *ini.File, command string, key string, currentValue int, flag string) int {
	if cfg.Section(command).HasKey(key) {
		value, _ := cfg.Section(command).Key(key).Int()
		if !cmd.Flags().Changed(flag) {
			return value
		}
	}
	return currentValue
}

func getBoolParameter(cmd *cobra.Command, cfg *ini.File, command string, key string, currentValue bool, flag string) bool {
	if cfg.Section(command).HasKey(key) {
		value, _ := cfg.Section(command).Key(key).Bool()
		if !cmd.Flags().Changed(flag) {
			return value
		}
	}
	return currentValue
}

func csvStringToIntArray(s string) []int {
	log.Debug("Converting to int array:", s)
	var retint []int
	if s != "" {
		r := csv.NewReader(strings.NewReader(s))
		intStr, err := r.Read()
		if err != nil {
			log.Critical(err)
			os.Exit(1)
		}

		log.Debug("Int fields found:\n")
		for _, field := range intStr {
			code, err := strconv.Atoi(field)
			if err != nil {
				log.Critical(err)
				os.Exit(1)
			}
			retint = append(retint, code)
			log.Debug(code)
		}
	}
	return retint
}

func csvStringToStringArray(s string) []string {
	log.Debug("Converting to string array:", s)
	var retstring []string
	if s != "" {
		r := csv.NewReader(strings.NewReader(s))
		stringsStr, err := r.Read()
		if err != nil {
			log.Critical(err)
			os.Exit(1)
		}
		log.Debug("Strings Found:\n")
		for _, field := range stringsStr {
			log.Debug(field)
			retstring = append(retstring, field)
		}
	}
	return retstring
}
func csvStringToRegexpArray(s string) []*regexp.Regexp {
	log.Debug("Converting to regexp array:", s)
	var retRegexps []*regexp.Regexp
	if s != "" {
		r := csv.NewReader(strings.NewReader(s))
		regexpStr, err := r.Read()
		if err != nil {
			log.Critical(err)
			os.Exit(1)
		}
		log.Debug("Regexp Strings Found:\n")
		for _, field := range regexpStr {
			log.Debug(field)
			retRegexps = append(retRegexps, regexp.MustCompile(field))
		}
	}
	return retRegexps
}

func loadParameters(cmd *cobra.Command, command string, iniFile string, expression string, retries int, duration int, retryOnAll bool, retryOnExitCodes string, retryOnStringMatches string, retryOnRegexpMatches string, successOnExitCodes string, successOnStringMatches string, successOnRegexpMatches string, performOnFailure string, failOnStringMatches string, failOnRegexpMatches string, failUnlessStringMatches string, failUnlessRegexpMatches string, printRetryOnFailure bool, printVerboseRetryOnFailure bool) (string, int, int, bool, []int, []string, []*regexp.Regexp, []int, []string, []*regexp.Regexp, string, []string, []*regexp.Regexp, []string, []*regexp.Regexp, bool, bool) {
	// Configure the defaxwult location of the INI file
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

	// Read the INI file - Local values override global valuesds
	cfg, err := ini.Load(loadIniFile)
	if err != nil {
		if iniFile != "" {
			log.Critical("Fail to read file: ", err)
			os.Exit(1)
		}
		log.Warning("Fail to read file: ", err)
	} else {
		log.Info("Loaded INI file:", loadIniFile)
		log.Info("Loading configuration settings for:", command)

		// By default, command line parameters come first...

		// If  not defined there, check the global section
		expression = getStringParameter(cmd, cfg, "", "expression", expression, "expression")
		retries = getIntParameter(cmd, cfg, "", "retries", retries, "retries")
		duration = getIntParameter(cmd, cfg, "", "duration", duration, "duration")

		log.Debug("After Loading Global INI Settings:")
		log.Debug("Expression: ", expression)
		log.Debug("Retries: ", retries)
		log.Debug("Duration: ", duration)
		log.Debug("Retry On All: ", retryOnAll)
		log.Debug("Retry On Exit Codes: ", retryOnExitCodes)
		log.Debug("Retry On String Matches: ", retryOnStringMatches)
		log.Debug("Retry On Regexp Matches: ", retryOnRegexpMatches)
		log.Debug("Success On Exit Codes: ", successOnExitCodes)
		log.Debug("Success On String Matches: ", successOnStringMatches)
		log.Debug("Success On Regexp Matches: ", successOnRegexpMatches)
		log.Debug("Perform On Failure: ", performOnFailure)
		log.Debug("Fail On String Matches: ", failOnStringMatches)
		log.Debug("Fail On Regexp Matches: ", failOnRegexpMatches)
		log.Debug("Fail Unless String Matches: ", failUnlessStringMatches)
		log.Debug("Fail Unless Regexp Matches: ", failUnlessRegexpMatches)
		log.Debug("Print Retry On Failure: ", printRetryOnFailure)
		log.Debug("Print Verbose Retry On Failure: ", printVerboseRetryOnFailure)

		// If anything is defined in the local section, override
		// cmd.Flags().IsSet()
		retryOnExitCodes = getStringParameter(cmd, cfg, command, "retry_on_exit_codes", retryOnExitCodes, "retry-on-exit-codes")
		retryOnStringMatches = getStringParameter(cmd, cfg, command, "retry_on_string_matches", retryOnStringMatches, "retry-on-string-matches")
		retryOnRegexpMatches = getStringParameter(cmd, cfg, command, "retry_on_regexp_matches", retryOnRegexpMatches, "retry-on-regexp-matches")
		successOnExitCodes = getStringParameter(cmd, cfg, command, "success_on_exit_codes", successOnExitCodes, "success-on-exit-codes")
		successOnStringMatches = getStringParameter(cmd, cfg, command, "success_on_string_matches", successOnStringMatches, "success-on-string-matches")
		successOnRegexpMatches = getStringParameter(cmd, cfg, command, "success_on_regexp_matches", successOnRegexpMatches, "success-on-regexp-matches")
		performOnFailure = getStringParameter(cmd, cfg, command, "perform_on_failure", performOnFailure, "perform-on-failure")
		failOnStringMatches = getStringParameter(cmd, cfg, command, "fail_on_string_matches", failOnStringMatches, "fail-on-string-matches")
		failOnRegexpMatches = getStringParameter(cmd, cfg, command, "fail_on_regexp_matches", failOnRegexpMatches, "fail-on-regexp-matches")
		failUnlessStringMatches = getStringParameter(cmd, cfg, command, "fail_unless_string_matches", failUnlessStringMatches, "fail-unless-string-matches")
		failUnlessRegexpMatches = getStringParameter(cmd, cfg, command, "fail_unless_regexp_matches", failUnlessRegexpMatches, "fail-unless-regexp-matches")
		retryOnAll = getBoolParameter(cmd, cfg, command, "retry_on_all", retryOnAll, "retry-on-all")
		expression = getStringParameter(cmd, cfg, command, "expression", expression, "expression")
		retries = getIntParameter(cmd, cfg, command, "retries", retries, "retries")
		duration = getIntParameter(cmd, cfg, command, "duration", duration, "duration")
		printRetryOnFailure = getBoolParameter(cmd, cfg, command, "print_retry_on_failure", printRetryOnFailure, "print-retry-on-failure")
		printVerboseRetryOnFailure = getBoolParameter(cmd, cfg, command, "print_verbose_retry_on_failure", printVerboseRetryOnFailure, "print-verbose-retry-on-failure")


		log.Debug("After Loading Local INI Settings:")
		log.Debug("Expression: ", expression)
		log.Debug("Retries: ", retries)
		log.Debug("Duration: ", duration)
		log.Debug("Retry On All: ", retryOnAll)
		log.Debug("Retry On Exit Codes: ", retryOnExitCodes)
		log.Debug("Retry On String Matches: ", retryOnStringMatches)
		log.Debug("Retry On Regexp Matches: ", retryOnRegexpMatches)
		log.Debug("Success On Exit Codes: ", successOnExitCodes)
		log.Debug("Success On String Matches: ", successOnStringMatches)
		log.Debug("Success On Regexp Matches: ", successOnRegexpMatches)
		log.Debug("Perform On Failure: ", performOnFailure)
		log.Debug("Fail On String Matches: ", failOnStringMatches)
		log.Debug("Fail On Regexp Matches: ", failOnRegexpMatches)
		log.Debug("Fail Unless String Matches: ", failUnlessStringMatches)
		log.Debug("Fail Unless Regexp Matches: ", failUnlessRegexpMatches)
		log.Debug("Print Retry On Failure: ", printRetryOnFailure)
		log.Debug("Print Verbose Retry On Failure: ", printVerboseRetryOnFailure)
	}

	// If after checking everywhere, the values are still -1, set them to their defaults
	/*
		if expression == "" {
			expression = _realExpressionDefault
		}

		if retries == -1 && duration == -1 {
			retries = _realRetryDefault
			duration = _realMaxDurationDefault
		}d
	*/

	// Treat the retry codes list as a row from a CSV file so we don't need to do intelligent parsing
	// var ignoreExitCodes []int
	log.Debug("Converting retyOnExitCodes...")
	ignoreExitCodes := csvStringToIntArray(retryOnExitCodes)
	log.Debug("Converting successOnExitCodes...")
	successExitCodes := csvStringToIntArray(successOnExitCodes)

	// Treat the retry strings list as a row from a CSV file so we don't need to do intelligent parsing
	// var ignoreStrings []string
	log.Debug("Converting retryOnStringMatches...")
	ignoreStrings := csvStringToStringArray(retryOnStringMatches)
	log.Debug("Converting successOnStringMatches...")
	successStrings := csvStringToStringArray(successOnStringMatches)

	// Treat the retry strings list as a row from a CSV file so we don't need to do intelligent parsing
	log.Debug("Converting retryOnRegexpMatches...")
	ignoreRegexps := csvStringToRegexpArray(retryOnRegexpMatches)
	log.Debug("Converting successOnRegexpMatches...")
	successRegexps := csvStringToRegexpArray(successOnRegexpMatches)

	log.Debug("Converting failOnStringMatches...")
	failOnStrings := csvStringToStringArray(failOnStringMatches)
	log.Debug("Converting failOnRegexpMatches...")
	failOnRegexps := csvStringToRegexpArray(failOnRegexpMatches)

	log.Debug("Converting failUnlessStringMatches...")
	failUnlessStrings := csvStringToStringArray(failUnlessStringMatches)
	log.Debug("Converting failUnlessRegexpMatches...")
	failUnlessRegexps := csvStringToRegexpArray(failUnlessRegexpMatches)

	log.Info("Fail On String Matches: ", failUnlessStringMatches)
	log.Info("Fail On String Matches: ", failUnlessStrings)

	return expression, retries, duration, retryOnAll, ignoreExitCodes, ignoreStrings, ignoreRegexps, successExitCodes, successStrings, successRegexps, performOnFailure, failOnStrings, failOnRegexps, failUnlessStrings, failUnlessRegexps, printRetryOnFailure, printVerboseRetryOnFailure
}

// ExponentialBackoff this is a separate function because perhaps somebody wants to run this
// without calling the command line in their golang code
func ExponentialBackoff(command []string, expression string, retries int, duration int, retryOnAll bool, ignoreExitCodes []int, ignoreStrings []string, ignoreRegexps []*regexp.Regexp, successExitCodes []int, successStrings []string, successRegexps []*regexp.Regexp, performOnFailure string, failOnStrings []string, failOnRegexps []*regexp.Regexp, failUnlessStrings []string, failUnlessRegexps []*regexp.Regexp, printRetryOnFailure bool, printVerboseRetryOnFailure bool) int {

	log.Info("-------- Settings -------")
	log.Info("Expression               : ", expression)
	log.Info("Retries                  : ", retries)
	log.Info("Duration                 : ", duration)
	log.Info("Retry On All             : ", retryOnAll)
	log.Info("Retry On Exit Codes      : ", ignoreExitCodes)
	log.Info("Retry On String Matches  : ", ignoreStrings)
	log.Info("Retry On Regexp Matches  : ", ignoreRegexps)
	log.Info("Success On Exit Codes    : ", successExitCodes)
	log.Info("Success On String Matches: ", successStrings)
	log.Info("Success On Regexp Matches: ", successRegexps)
	log.Info("Perform On Failure       : ", performOnFailure)
	log.Info("Fail On String Matches: ", failOnStrings)
	log.Info("Fail On Regexp Matches: ", failOnRegexps)
	log.Info("Fail Unless String Matches: ", failUnlessStrings)
	log.Info("Fail Unless Regexp Matches: ", failUnlessRegexps)
	log.Info("Print Retry On Failure: ", printRetryOnFailure)
	log.Info("Print Verbose Retry On Failure: ", printVerboseRetryOnFailure)
	log.Info("Command to Run           : ", command)
	log.Info("-------------------------")

	xIncrement := 0
	start := time.Now()
	for {
		log.Debug("Running:", command[0])
		log.Debug("Params:", command[1:len(command)])
		cmd := exec.Command(command[0], command[1:len(command)]...)
		var out bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &stderr
		cmd.Run()
		exitCode := cmd.ProcessState.ExitCode()
		log.Debug("Command exitted with ", exitCode)
		needToExit := true

		// Automatic failure if certain string is matched
		for i := range failOnStrings {
			if strings.Contains(out.String(), failOnStrings[i]) {
				log.Debug("Output stream contained: ", failOnStrings[i], ". Converting exit code to -1.")
				exitCode = -1
				needToExit = false
			}
			if strings.Contains(stderr.String(), failOnStrings[i]) {
				log.Debug("Error stream contained: ", failOnStrings[i], ". Converting exit code to -1.")
				exitCode = -1
				needToExit = false
			}
		}

		// Automatic failure if certain regexp is matched
		for i := range failOnRegexps {
			if failOnRegexps[i].MatchString(out.String()) {
				log.Debug("Output stream contained regexp: ", failOnRegexps[i], ". Converting exit code to -1.")
				exitCode = -1
				needToExit = false
			}
			if failOnRegexps[i].MatchString(stderr.String()) {
				log.Debug("Error stream contained regexp: ", failOnRegexps[i], ". Converting exit code to -1.")
				exitCode = -1
				needToExit = false
			}
		}

		if len(failUnlessStrings) > 0 {
			exitCode = -1
			needToExit = false
			for i := range failUnlessStrings {
				if strings.Contains(out.String(), failUnlessStrings[i]) {
					log.Debug("Output stream contained: ", failUnlessStrings[i], ". Converting exit code to 0.")
					needToExit = true
					exitCode = 0
				}
				if strings.Contains(stderr.String(), failUnlessStrings[i]) {
					log.Debug("Error stream contained: ", failUnlessStrings[i], ". Converting exit code to 0.")
					needToExit = true
					exitCode = 0
				}
			}
		}
		if len(failUnlessRegexps) > 0 {
			exitCode = -1
			for i := range failUnlessRegexps {
				if failUnlessRegexps[i].MatchString(out.String()) {
					log.Debug("Output stream contained regexp: ", failUnlessRegexps[i], ". Converting exit code to 0.")
					needToExit = true
					exitCode = 0
				}
				if failUnlessRegexps[i].MatchString(stderr.String()) {
					log.Debug("Error stream contained regexp: ", failUnlessRegexps[i], ". Converting exit code to 0.")
					needToExit = true
					exitCode = 0
				}
			}
		}

		for i := range successExitCodes {
			if exitCode != 0 && exitCode == successExitCodes[i] {
				log.Debug("Program exited with code: ", exitCode, ". Converting to exit code to 0.")
				exitCode = 0
			}
		}

		// Do not exit if output / stderr from the command contained a string in our retryOnMatchedStrings list
		for i := range successStrings {
			if exitCode != 0 && strings.Contains(out.String(), successStrings[i]) {
				log.Debug("Output stream contained: ", successStrings[i], ". Converting exit code to 0.")
				exitCode = 0
			}
			if exitCode != 0 && strings.Contains(stderr.String(), successStrings[i]) {
				log.Debug("Error stream contained: ", successStrings[i], ". Converting exit code to 0.")
				exitCode = 0
			}
		}

		// Do not exit if output / stderr from the command contained a string in our retryOnMatchedStrings list
		for i := range successRegexps {
			if exitCode != 0 && successRegexps[i].MatchString(out.String()) {
				log.Debug("Output stream contained regexp: ", successRegexps[i], ". Converting exit code to 0.")
				exitCode = 0
			}
			if exitCode != 0 && successRegexps[i].MatchString(stderr.String()) {
				log.Debug("Error stream contained regexp: ", successRegexps[i], ". Converting exit code to 0.")
				exitCode = 0
			}
		}

		// Do not exit if the exit code matched our input retryOnExitCodes
		if exitCode != 0 {
			if needToExit && retryOnAll {
				log.Debug("Program exited with code: ", exitCode, ". Restarting on all non-zero exit codes.")
				needToExit = false
			}

			for i := range ignoreExitCodes {
				if needToExit && exitCode == ignoreExitCodes[i] {
					log.Debug("Program exited with code: ", exitCode, ". Restarting.")
					needToExit = false
				}
			}

			// Do not exit if output / stderr from the command contained a string in our retryOnMatchedStrings list
			for i := range ignoreStrings {
				if needToExit && strings.Contains(out.String(), ignoreStrings[i]) {
					log.Debug("Output stream contained: ", ignoreStrings[i], ". Restarting.")
					needToExit = false
				}
				if needToExit && strings.Contains(stderr.String(), ignoreStrings[i]) {
					log.Debug("Error stream contained: ", ignoreStrings[i], ". Restarting.")
					needToExit = false
				}
			}

			// Do not exit if output / stderr from the command contained a string in our retryOnMatchedStrings list
			for i := range ignoreRegexps {
				if needToExit && ignoreRegexps[i].MatchString(out.String()) {
					log.Debug("Output stream contained regexp: ", ignoreRegexps[i], ". Restarting.")
					needToExit = false
				}
				if needToExit && ignoreRegexps[i].MatchString(stderr.String()) {
					log.Debug("Error stream contained regexp: ", ignoreRegexps[i], ". Restarting.")
					needToExit = false
				}
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
		log.Debug("Elapsed Time:", elapsed)

		if xIncrement > retries && retries != -1 {
			log.Warning("Failed to complete command due to retries exhausted:", command)
			log.Warning("Exitting with error code:", exitCode)
			os.Stderr.WriteString(stderr.String())
			fmt.Print(out.String())
			return exitCode
		}

		log.Debug("Time check:", elapsed, ">=", time.Duration(duration)*time.Second, "?")
		if elapsed >= time.Duration(duration)*time.Second && duration != -1 {
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
		//time.Duration will round to whatever it is multiplied by... do not switch to time.Second
		sleepForD := time.Duration(value*1000) * time.Millisecond
		log.Debug("Planning to sleep for", sleepForD)

		t = time.Now()
		elapsed = t.Sub(start)
		// If our max wait is 600 seconds, we've waited 596, and our next wait duration is 30,
		// do some math so we don't go over 600 seconds
		log.Debug("Overrun check:", (elapsed + sleepForD), ">=", time.Duration(duration)*time.Second)
		if elapsed+sleepForD >= time.Duration(duration)*time.Second && duration != -1 {
			sleepForD = time.Duration(duration)*time.Second - elapsed
			log.Debug("Adjusted Sleep Due To Max Overrun:", sleepForD)
		}
		log.Info("Time to sleeping for before retrying: ", sleepForD)

		if (printRetryOnFailure || printVerboseRetryOnFailure) {
			if (printVerboseRetryOnFailure) {
				os.Stderr.WriteString(stderr.String())
				fmt.Print(out.String())
			}
			fmt.Println("Next Retry Attempt",xIncrement,"in",sleepForD,"...")
		}
		time.Sleep(sleepForD)

		if performOnFailure != "" {
			log.Debug("Parsing:", performOnFailure)
			errorCommand, err := shellwords.Parse(performOnFailure)
			if err != nil {
				log.Error("Unable to parse Perform On Failure command input:", performOnFailure)
				log.Critical(err)
				os.Exit(1)
			}

			log.Debug("Perform On Failure Command string parsed as:\n")
			for _, field := range errorCommand {
				log.Debug(field)
			}

			log.Debug("Running:", errorCommand[0])
			log.Debug("Params:", errorCommand[1:len(errorCommand)])
			ecmd := exec.Command(errorCommand[0], errorCommand[1:len(errorCommand)]...)
			var eout bytes.Buffer
			var estderr bytes.Buffer
			ecmd.Stdout = &eout
			ecmd.Stderr = &estderr
			ecmd.Run()
			eexitCode := ecmd.ProcessState.ExitCode()
			log.Debug("Command exitted with ", eexitCode)
			log.Info(estderr.String())
			log.Info(eout.String())
			if eexitCode != 0 {
				log.Error("Unable To Run Perform On Failure command:", performOnFailure)
				log.Critical(err)
				os.Exit(1)
			}
		}
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
	rootCmd.PersistentFlags().StringVarP(&_iniFile, "ini-file", "f", "", "An INI file to load with tool settings (default $HOME/.eb.ini)\nThe INI file supports global and local parameters\nLocal parameters override global parameters")
	rootCmd.PersistentFlags().StringVarP(&_expression, "expression", "e", "0", "A mathmematical expression representing the time to wait on each retry\nThe variable 'x' is the current iteration (0 based)\nThe variable 'i' is the current iteration (1 based)\nThe variable 'r' is a random float from 0-1\nExamples: \"x*15+15\", \"x*x\", \"(x*x)+(10*r)\"")
	rootCmd.PersistentFlags().IntVarP(&_retries, "retries", "r", -1, "The number of times to retry the command")
	rootCmd.PersistentFlags().IntVarP(&_duration, "duration", "d", -1, "How many seconds to keep retrying")
	rootCmd.PersistentFlags().BoolVarP(&_retryOnAll, "retry-on-all", "a", false, "Retry on all non-zero exit codes")
	rootCmd.PersistentFlags().StringVarP(&_performOnFailure, "perform-on-failure", "p", "", "A command to run prior to retrying the command")
	rootCmd.PersistentFlags().BoolVarP(&_printRetryOnFailure, "print-retry-on-failure", "m", false, "Print a simple retrying message prior to retrying")
	rootCmd.PersistentFlags().BoolVarP(&_printVerboseRetryOnFailure, "print-verbose-retry-on-failure", "M", false, "Print a verboose retrying message prior to retrying")
	rootCmd.PersistentFlags().StringVarP(&_retryOnExitCodes, "retry-on-exit-codes", "c", "", "A comma delimited list of exit codes to try on")
	rootCmd.PersistentFlags().StringVarP(&_retryOnStringMatches, "retry-on-string-matches", "s", "", "A comma delimited list of strings found in stderr or stdout to retry on")
	rootCmd.PersistentFlags().StringVarP(&_retryOnRegexpMatches, "retry-on-regexp-matches", "x", "", "A comma delimited list of regular expressions found in stderr or stdout to retry on")
	rootCmd.PersistentFlags().StringVarP(&_successOnExitCodes, "success-on-exit-codes", "C", "", "A comma delimited list of exit codes to change to success codes")
	rootCmd.PersistentFlags().StringVarP(&_successOnStringMatches, "success-on-string-matches", "S", "", "A comma delimited list of strings to change to success codes")
	rootCmd.PersistentFlags().StringVarP(&_successOnRegexpMatches, "success-on-regexp-matches", "X", "", "A comma delimited list of regular expressions to change to success codes")
	rootCmd.PersistentFlags().StringVarP(&_failOnStringMatches, "fail-on-string-matches", "o", "", "A comma delimited list of strings to consider failures to retry on")
	rootCmd.PersistentFlags().StringVarP(&_failOnRegexpMatches, "fail-on-regexp-matches", "O", "", "A comma delimited list of regular expressions to consider failures to retry on")
	rootCmd.PersistentFlags().StringVarP(&_failUnlessStringMatches, "fail-unless-string-matches", "u", "", "A comma delimited list of strings consider successful. Fail otherwise")
	rootCmd.PersistentFlags().StringVarP(&_failUnlessRegexpMatches, "fail-unless-regexp-matches", "U", "", "A comma delimited list of regular expressions to consider successful. Fail otherwise")
	rootCmd.PersistentFlags().BoolVarP(&_verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().BoolVar(&_version, "version", false, "Print the version and exit")
	rootCmd.PersistentFlags().BoolVarP(&_kill, "kill", "k", false, "Immediately exit with a .75 probability (for testing failures)")
	rootCmd.PersistentFlags().BoolVarP(&_debug, "debug", "g", false, "Enable debugging")
}
