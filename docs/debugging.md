## Debugging the Repository 

This document provides a guide on how to debug the repository using Visual Studio Code (VSCode). It is important to note that to effectively debug, a `launch.json` file must be added to the `.vscode` folder. 

Below is a sample `launch.json` file that can be used to debug the `validate` command:

```json
{
    "version": "0.2.0",
    "configurations": [
        {
            "name": "Validate",
            "type": "go",
            "request": "launch",
            "mode": "debug",
            "program": "main.go",
            "showGlobalVariables": true,
            "dlvFlags": [],
            "cwd": "path-to-charts-folder",
            "env": {},
            "output": "scripts_debug",
            "args": [
                "validate"
            ]
        },
    ]
}
```
## Understanding the Configurations Attribute
The configurations attribute is an array of distinct configurations that can be used to debug. These configurations appear in the dropdown menu of the debug section on VSCode.

If you are using an old version of Go, i.e., 1.16, you should add --check-go-version=false into your dlvFlags.

## The cwd Attribute
The cwd attribute is the working directory against which the project will run. An absolute path to your local charts repository should be provided here.

## The env Attribute
The env attribute is an array containing all the environment variables that will be used during the debug session.

## The output Attribute
The output attribute denotes the name of the Go binary that will be generated.

## The args Attribute
The args attribute is an array of arguments that the go run command will take to run the debug session.