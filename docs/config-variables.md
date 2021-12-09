# Configuration Variables

Pipeline configuration files support variables in their values which can be
replaced from a variety of sources. If a YAML document specifies a variable,
it will be deemed as required and the build will fail if no suitable replacement
is found.

## Format

Variables can be defined in the configuration files using bash-like embedded 
code: `${VAR_NAME}`. All variable names must be uppercase. Names con include
letters (A-Z), digits (0-9) and the underscore (_). Variables **MUST** be 
prefixed with `$` and enclosed between curly braces: `{}`.

A variable can be used multiple times and **MUST** be used only in the YAML
values, not in the identifiers (this is not enforced but will, most likely 
result in invalid YAML if atempted).

### Variable Example:

```yaml
transfers:
  - source: ["mattermost-webapp.tar.gz"]
    destination: s3://${BUCKET}/gitlab/${PROJECT_NAME}/ee/test/${COMMIT_SHA}
  - source: ["mattermost-webapp.tar.gz"]
    destination: s3://${BUCKET}/gitlab/${PROJECT_NAME}/te/${COMMIT_SHA}
```

This snippet defines three variables: `BUCKET`, `PROJECT_NAME` and `COMMIT_SHA`.

## Value Sources

The values for variables can be defined from three main sources:

1. **env: section of the configuration file:**<br>
If the document has an `env:` section (defining environment variables) with a 
value matching the variable name, the value will be used as part of the
configuration file. *Note:* a configuration file can define a variable without
a value in its env section. In this case, the config variable value will be
set to an empty string ("") and the build will not fail.

1. **System enviroment:**<br>
If no value for the variable was found in the configuration file, the build
system will perform a lookup of the system environment variables. If a matching 
env var is set, its value will be used in the config file. If a value was
set in the env section of the conf file, no lookup will be performed unless the
value is an empty string.

1. **Secrets**<br>
Variables can be read from secrets. This is not yet implemented yet.

