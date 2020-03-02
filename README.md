# Exponential Backoff Tool
### Introduction
Exponential Backoff Tool is a command line tool for performing intelligent retries based on the output of a shell command.

This tool is used to wrap tools that require an API to connect to (such as git, gcloud, vault), and build in intelligent exponential backoff. The tool can accept an expression to support any kind of exponential backoff (such as, truely exponential, x*x, or incremental, such as x*15). It also supports staggering commands by a random interval if they fail.

### Motivation
With the rise of microservices and an ever increasing cloud-reliant environment, the ability to tolerate failure is imperative. Tools such as aws, gcloud, and git, do not support built-in exponential backoff. For example, a complicate Jenkins Job could make dozens of calls to external API servers, and if any of the API servers are offline, the Jenkins job would fail. There are often multiple kinds of failures, and exit codes may not be readily available, if even usable (Google states developers should not place too many exit codes in their code).
```
Individual APIs should avoid defining additional error codes, since developers are very unlikely to write logic to handle a large number of error codes. For reference, handling an average of 3 error codes per API call would mean most application logic would just be for error handling, which would not be a good developer experience. - https://cloud.google.com/apis/design/errors
```

However, Google is stating which errors clients should retry. It is not up to Google, nor the developer, to decide which API calls should be retried.

For example, in certain cases, "Rate Limit Exceeded" can, and should be treated in the same manner as "Quota exceeded". For example, this can be retried using exponential backoff.
```
ERROR: (gcloud.sql.reschedule-maintenance) RESOURCE_EXHAUSTED: Quota exceeded for quota group 'default' and limit 'Queries per user per 100 seconds' of service 
```
Other quota exhaustion errors, such as Quota Exceeded for creating new Cloud SQL instances, cannot be retried.
 
Exit codes cannot be trusted because applications tend to throw exceptions that are caught in a central main loop, followed by `os.Exit(1)`. For example:
```
try {
    myEntireApplication();
} catch (Exception e) {
    e.printStackTrace();
    System.exit(1);
}
```

### Documentation
##### Usage
The comamnd can be used as follows.
`eb <command> [flags]`

The `<command>` can be passed in without quotas, such as `eb curl www.google.com` provided  none of the below flags are duplicated. If a command contains a flag below, the command can be passed in using by using quotes, such as `eb "curl -f www.google.com"`

This command will provide the original exit code from the command running. 

##### Flags
* `-d, --debug`
Enable Debugging
* `-t, --duration`
*(Integer)* How long to keep retrying for (Default: 0)
* `-e, --expression`
*(String)* A mathmematical expression representing the time to wait on each retry (Default: "0").
The variable 'x' is the current iteration (0 based).
The variable 'i' is the current iteration (1 based).
The variable 'r' is a random float from 0-1.
Examples: "x*15+15", "x*x", "(x*x)+(10*r)"
* `-h, --help`
Print the help screen
* `-f, --ini-file`
*(String)* An INI file to load with tool settings (Default: $HOME/.eb.ini)
The INI file supports global and local parameters.
Local parameters override global parameters.
* `-r, --retries`
*(Intger)* The number of times to retry the command (Default: 0)
* `-c, --retry-on-exit-codes`
*(String)* A comma delimited list of exit codes to try on.
* `-s, --retry-on-string-matches`
A comma delimited list of strings found in stderr or stdout to retry on.
* `-v, --verbose` Enable Verbose Output
 
##### Sample INI File
Command line parameters override the INI file. Local sections of the INI file override global sections of the INI file. expression: "15*i"
```
###########################################################
# eb.ini
# This is the global section. This applies to all commands.
# If a command line parameter is passed in, the respective
# parameter in this file will be ignored (command line)
# takes precedence.
###########################################################

# Expression defines the mathematical algorithm for backing 
# off. It supports 'x', 'i', and 'r'. x is the current 
# attempt (0 based), i is the current attempt (1 based), 
# and r is a random float between 0 and 1 (to allow for .
# jitter)
# expression: "15*i"

# The maximum number of retries to make.
# retries: 30

# The maximum amount of time to allow for this to retry for.
# duration: 30

###########################################################
# eb.ini
# This is the local section. This applies to specific 
# commands. This will override the global section.
###########################################################


# Use brackets to adjust the settings for a specific command.
# [curl]

# Different values for global variables can be passed in
# to apply to just this command.
# expression: "x*x"
# retries: 15
# duration: 60

# If the following exit code is returned, retry the command.
# This is comma-delimited. The values "1,2,3" and "1","2","3"
# are synonymous.
# retry_on_exit_codes: "4,5,6"

# If the following text is found in either the command 
# strout, or strerr, retry the command. This is comma
# delimited. The values "1,2,3" and "1","2","3" are
# synonymous.
# retry_on_string_matches: "Could not resolve host:"
```

### Examples
Running the `eb` without any parameters is like running the original `command` as-is.
```
$ eb kubectl get pods
Unable to connect to the server: dial tcp 127.0.0.1:443: i/o timeout
```

This will retry 10 times, waiting 1,1,1,1...1 seconds between each iteration
```
$ eb kubectl get pods -e "1" -r 10 -s "Unable to connect to the server"
```

This will retry 10 times, waiting 0,1,4,9...100 seconds between each iteration
```
$ eb kubectl get pods -e "x*x" -r 10 -s "Unable to connect to the server"
```

This will retry 10 times, waiting 0,15,30,45...135 seconds between each iteration
```
$ eb kubectl get pods -e "x*15" -r 10 -s "Unable to connect to the server"
```

This will retry 10 times, waiting 15,30,45,60...150 seconds between each iteration
```
$ eb kubectl get pods -e "i*15" -r 10 -s "Unable to connect to the server"
```

This will retry 10 times, waiting 15,30,45,60...150 seconds between each iteration, each time adding an an extra random 0-5 seconds between each retry.
```
$ eb kubectl get pods -e "i*15+5*r" -r 10 -s "Unable to connect to the server"
```

This will keep retrying for 10 seconds,  waiting 15,30,45,60...150 seconds between each iteration, each time adding an an extra random 0-5 seconds between each retry.
```
$ eb kubectl get pods -e "i*15+5*r" -t 10 -s "Unable to connect to the server"
```

This will retry either 10 times or for 10 seconds whichever comes first, waiting 15,30,45,60...150 seconds between each iteration, each time adding an an extra random 0-5 seconds between each retry.
```
$ eb kubectl get pods -e "i*15+5*r" -r 10 -t 10 -s "Unable to connect to the server"
```