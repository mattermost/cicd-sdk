# Build Package

The build package astracts a software build process. It is intended
to be pluggable, repeatable, and with enough introspection capabilities
to enable querying its state at any point.

The build package implements two abstractions to work: A runner and a run.

## Runner

A runner is an interface that abstracts the process of software building.
It takes some inputs, performs its job during the main method (`Runner.Execute`)
and hopefully produces some outputs.

To envision how a runner works, we can think about the first object 
that implements the interface: `build.Make`. The Make runner gets some arguments,
passes them to the `make` command and returns the execution status. That's 
it.

The runner interface is designed to be easy to implement by other processes
in the future: npm, docker, etc.

## Run

A run is an object that calls the `Execute()` method of a runner. Its job is to 
be keep track of the execution, make the run details available to query and
transform the state and metadata into other formats.

## Example Usage

```golang

// Create a build with a make runner. This is equivalent
// to running `make my-x86-target` in the current directory:
b := build.New(runners.NewMake("test-target"))

// You can set the working directory
b.Options().Workdir = "tmp/make"

// Generate a run and execute it:
b.Run().Execute()

// Launch a new Run of our build 
run := build.Run()

// We can query the run to see if it was successful:
if run.Successful() {
    fmt.Println("Run %d finished without errors", run.ID)
}

// we can print the output:
fmt.Println(run.Output())

// If we launch another run. We run the exact command again
run := build.Run()

// In an ideal world, we should get the same output. We should
// thrive to get reproducible builds.

// We can query the build to get historical data of the builds:
for i := range build.Runs {
    fmt.Println("Build #%d finished at %s", build.Runs[i].ID, build.Runs[i].EndTime.String())
}


```