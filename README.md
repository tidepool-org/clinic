# Clinic Service
### Overview

The Clinic Service manages Clinics and there relationships between patients and clinicians.
This service is a proof of concept for attempting to use more open source software
within our services to minimize code and cut down on the boilerplate.

The overarching goal is to go from JSON Open Schema specification to server with a minimum 
amount of code.  We store the schema files in https://github.com/tidepool-org/TidepoolApi
as yaml files.

The main open source software packages that we use are:

* https://github.com/tidepool-org/oapi-codegen for code generation.  This takes the yaml file 
the API repo and generates a server implementation, a set of data types and a swagger 
specification
* https://github.com/getkin/kin-openapi for validating that input (and responses) into 
(and out of) the server are valid.  This code is used as middleware for the web framework
* https://github.com/labstack/echo as a web framework to minimize all the code to run a
service
* https://github.com/mongodb/mongo-go-driver - the latest mongo drivers which enables easier
reading and writing from go structs directly to database.

### Code generation

The OpenAPI Client and Server Code Generation tool generates the necessary files from
a bundled yaml file exported from Spotlight Studio.  Unfortunately, Spotlight Studio 
exports the yaml file in a slightly different format than what the code generation tool
can import.  We had to write a python script - fixYaml.py to move the references to the components 
section of the schema for the code generation tool to work.

There is also a shell script - refreshYaml.sh which will do everything necessary to 
regenerate the server, types and swagger files.  If minor changes were made (such as 
just adding more validation) - this service will continue to work.  If more major changes
are made (such as changing the data structures) - the service will also have to be 
modified.

### Makefile

The makefile has three targets:
* generate - code generation for server stubs, types and specification.  Runs intermediate 
scripts to match output of studio to oapigen
* test - runs test scripts
* build - builds package

