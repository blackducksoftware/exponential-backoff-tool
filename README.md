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
For example, the AWS SDK points to this document with regards to its handling of exceptions:
https://docs.oracle.com/javase/tutorial/essential/exceptions/runtime.html

### Documentation
##### Usage
The command can be used as follows.
`eb <command> [flags]`

The `<command>` can be passed in without quotas, such as `eb curl www.google.com` provided  none of the below flags are duplicated. If a command contains a flag below, the command can be passed in using by using quotes, such as `eb "curl -f www.google.com"`

This command will provide the original exit code from the command running. 

##### Flags
* `-g, --debug`
Enable debugging.
* `-d, --duration`
*(Integer)* How long to keep retrying for (Default: -1)
* `-b, --enable-metrics`
Enable collection of call metrics. The metrics are output as a a csv file, eb-metrics.csv.
* `-e, --expression`
*(String)* A mathmematical expression representing the time to wait on each retry (Default: "0").
The variable 'x' is the current iteration (0 based).
The variable 'i' is the current iteration (1 based).
The variable 'r' is a random float from 0-1.
Examples: "x*15+15", "x*x", "(x*x)+(10*r)"
* `-O, --fail-on-regexp-matches`
*(String)* A comma delimited list of regular expressions to consider failures to retry on.
* `-o, --fail-on-string-matches`
*(String)* A comma delimited list of strings to consider failures to retry on.
* `-U, --fail-unless-regexp-matches`
*(String)* A comma delimited list of regular expressions to consider successful. Fail otherwise.
* `-u, --fail-unless-string-matches`
*(String)* A comma delimited list of strings consider successful. Fail otherwise.
* `-h, --help`
Print the help screen
* `-f, --ini-file`
*(String)* An INI file to load with tool settings (Default: $HOME/.eb.ini)
The INI file supports global and local parameters.
Local parameters override global parameters.
* `-k, --kill`
Fail the `eb` 75% of the time without running anything. Useful in testing expressions or intermittent failures.
* `-P, --perform-on-exit string`
*(String)* A command to run prior to exiting. This command does not exponentially backoff and is intended for uploading performance metrics. This always runs regardless of whether the original command succeeds or fails.
* `-p, --perfom-on-failure`
*(String)* A command to run whenever the original command fails. This command does not exponentially backoff and is intended for cleanup to keep the original command working (such as a command that touchs a file when it runs with the intent of populating it, it fails, and then a subsequent run fails because the file was touched)
* `-r, --retries`
*(Integer)* The number of times to retry the command (Default: -1)
* `-a, --retry-on-all`
Retry on all non-zero exit codes.
* `-c, --retry-on-exit-codes`
*(String)* A comma delimited list of exit codes to try on.
* `-x, --retry-on-regexp-matches`
*(String)*A comma delimited list of regular expressions found in stderr or stdout to retry on.
* `-s, --retry-on-string-matches`
*(String)*A comma delimited list of strings found in stderr or stdout to retry on.
* `-C, --success-on-exit-codes`
*(String)* A comma delimited list of exit codes  to change to success codes.
* `-X, --success-on-regexp-matches`
*(String)*A comma delimited list of regular expressions found in stderr or stdout  to change to success codes.
* `-S, --success-on-string-matches`
*(String)*A comma delimited list of strings found in stderr or stdout  to change to success codes.
* `-v, --verbose` 
Enable Verbose Output.
* `--version` 
Print the version and exit.
 
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

# If any non-zero exit code is returned, retry the command.
# retry_on_all: "false"

# If the following exit code is returned, retry the command.
# This is comma-delimited. The values "1,2,3" and "1","2","3"
# are synonymous.
# retry_on_exit_codes: "4,5,6"

# If the following text is found in either the command 
# strout, or strerr, retry the command. This is comma
# delimited. The values "1,2,3" and "1","2","3" are
# synonymous.
# retry_on_string_matches: "Could not resolve host:"

# Perform on failure can be used to run a command to clean up
# whatever the root command performed. That could be removing
# a PID file, or in my use case, a file the gets 'touched', with
# no contents due to command failure, and then subsequent commands
# fail because the file exists
# perform_on_failure: "echo failed"

# Whether to collect metrics. The metrics are output as a a csv file, eb-metrics.csv.
# metrics_enabled: "true"

# Perform this command when the command succeeds or fails. Note that -P at the end
# to prevent EB from entering an infinite loop.
# perform_on_exit: "eb 'gsutil cp ./eb-metrics.csv gs://my-bucket/eb-metrics.csv' -P 'true'"

```

### Examples
Running the `eb` without any parameters is like running the original `command` as-is.
```
$ eb kubectl get pods
Unable to connect to the server: dial tcp 127.0.0.1:443: i/o timeout
```

The EB command can be conviently tested to see how different expressions work:
```
eb "eb -k" -c 1 -e "5*x" -r 2
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

This command will convert any success codes that contain a string to failures. This is useful for web requests that return 200 on failures (see S3 documentation). As a result, this command will fail and retry on messages such as "200 OK: Your web request failed".
```
$ eb "eb -k" -e 0 -o failed -m -a
...
...
Next Retry Attempt 35 in 0s ...
Next Retry Attempt 36 in 0s ...
Next Retry Attempt 37 in 0s ...
Sample Success: 200 OK: Your request succeeded
```

This command will convert any success codes that contain a regexp to failures. This is useful for web requests that return 200 on failures (see S3 documentation). As a result, this command will fail and retry on messages such as "200 OK: Your web request failed".
```
$ ./eb "./eb -k" -e "0" -O 200.*failed -a -m
...
...
Next Retry Attempt 116 in 0s ...
Next Retry Attempt 117 in 0s ...
Next Retry Attempt 118 in 0s ...
Sample Success: 200 OK: Your request succeeded
```

This command consides everything a failure unless it contains the string 'succeeded'.
```
$ eb "eb -k" -e "0" -u "succeeded" -m -a
...
Next Retry Attempt 155 in 0s ...
Next Retry Attempt 156 in 0s ...
Next Retry Attempt 157 in 0s ...
Sample Success: 200 OK: Your request succeeded
```

This command consides everything a failure unless it contains the regexp 'suc.*eded'.
```
$ eb "eb -k" -e "0" -U "suc.*eded" -m -a
...
Next Retry Attempt 155 in 0s ...
Next Retry Attempt 156 in 0s ...
Next Retry Attempt 157 in 0s ...
Sample Success: 200 OK: Your request succeeded
```


This command is useful for testing out retry logic. It will keep retrying until is sees 200.*succeeded in the console output.
```
$ while true; do eb "eb -k" -U "200.*succeeded" -e ".001*x" -a; done
Sample Success: 200 OK: Your request succeeded
Sample Success: 200 OK: Your request succeeded
Sample Success: 200 OK: Your request succeeded
Sample Success: 200 OK: Your request succeeded
Sample Success: 200 OK: Your request succeeded
Sample Success: 200 OK: Your request succeeded
```

Contrast the above command with the following command. Some web requests return 200 on failures which is considered successful (see S3 documentation) despite the fact that for you it is a failure.
```
$ while true; do eb "eb -k" -a -e ".001*x"; done
Sample Error: 200 OK: The web request failed
Sample Error: 200 OK: The web request failed
Sample Error: 200 OK: The web request failed
Sample Success: 200 OK: Your request succeeded
Sample Error: 200 OK: The web request failed
Sample Error: 200 OK: The web request failed
```

This command will keep retrying on all failures printing out only a simply "Retrying" message
```
$ eb "eb -k" -e ".001*x" -m -a
...
...
Next Retry Attempt 127 in 126ms ...
Next Retry Attempt 128 in 127ms ...
Next Retry Attempt 129 in 128ms ...
Sample Success: 200 OK: Your request succeeded
```

This command will keep retrying and print out any error messages that are received until the command succeeds. Note that in the below output, `Sample Error: 200 OK: The web request failed` actually returns 0 as a return code since web requests can get tricky.
```
$ eb "eb -k" -e ".001*x" -M -a
...
...
Sample Error: Request Timed Out
Next Retry Attempt 13 in 12ms ...
Sample Error: Request Timed Out
Next Retry Attempt 14 in 13ms ...
Sample Error: Concurrent Modification
Next Retry Attempt 15 in 14ms ...
Sample Error: 200 OK: The web request failed
```

This command will collect metrics, and then upload them to a cloud bucket when the command completes. Note the nested usage of `eb` could case an infinite loop, so we add `-P 'true'` to prevent this.
```
$ eb "echo this is some text" -b -P "eb 'gsutil cp ./eb-metrics.csv gs://my-bucket/eb-metrics.csv' -P 'true'"
this is some text
```


### Jenkins Example
Jenkins is really difficult to deal with when using quotes, and with `eb`, you may need multiple quotes. Here is an exmaple of that:
```
sh '''
PGPASSWORD=${PASSWORD} eb "psql -h 127.0.0.1 -U postgres postgres -c \\"CREATE USER my_user WITH password '${MY_PASSWORD}';\\""
'''
```

### Additional Tips
This tool was written with the parameters it has because they all have a use case I have specifically encountered.

For example, creating a gcloud instance can first fail with a "Too many API Requests". If retrying on this, a subsequent run of the command may result in "Cloud SQL Instance Already Exists"-- despite the fact that the first attempt was actually a failure.

```
ERROR: (gcloud.sql.instances.create) RESOURCE_EXHAUSTED: Quota exceeded for quota group 'default' and limit 'Queries per user per 100 seconds' of service 'sqladmin.googleapis.com' for consumer 'project_number:548026760868'.
ERROR: (gcloud.sql.instances.create) Resource in project [saas-hub-stg] is the subject of a conflict: The Cloud SQL instance already exists. When you delete an instance, you cannot reuse the name of the deleted instance until one week from the deletion date.
```

As a result, a combination of both `eb "gcloud sql instances create ..." --retry-on-string-matches "Quota exceeded for quota group" --success-on-string-matches "The Cloud SQL instance already exists." may be needed.