# Clinic Service 
### Overview

The Clinic Service manages Clinics and their patients and clinicians.

Server stubs and client library are generated from JSON Open Schema specification. We store the schema files in https://github.com/tidepool-org/TidepoolApi
as yaml files.

The main open source software packages that we use are:

* https://github.com/tidepool-org/oapi-codegen for code generation.  This takes the yaml file 
the API repo and generates a server implementation, a set of data types and a swagger 
specification
* https://github.com/getkin/kin-openapi for validating that input (and responses) into 
(and out of) the server are valid.  This code is used as middleware for the web framework
* https://github.com/labstack/echo as a web framework to minimize all the code to run a
service

### Asynchronous processing

To improve the resilience and minimize the direct dependencies to other services some operations
are handled asynchronously. The clinic administration services rely heavily on CDC (Change Data Capture) 
pattern to react to events and process event streams. We use [Kafka](https://kafka.apache.org/), 
[Kafka Connect](https://docs.confluent.io/3.0.1/connect/intro.html) and 
[Kafka Mongo Connector](https://docs.mongodb.com/kafka-connector/current/). The mongo connector for kafka
uses the mongo oplog as a source for events and writes a message to a kafka topic every time a document
in mongo is created, updated or deleted. There are two consumers of those events - 
the [clinic-worker](https://github.com/tidepool-org/clinic-worker) service and the mongo connector for kafka itself
which can read a CDC source stream and replay it to a different mongo collection. 

#### Permissions

Tidepool uses the [gatekeeper](https://github.com/tidepool-org/gatekeeper) service to determine 
whether one user has access to another user's data. However, permissions from patients to clinics 
and from clinics to clinicians are stored in the clinic service database. Those are asynchronously
replicated to gatekeeper's database using 
[Kafka Mongo Connector](https://docs.mongodb.com/kafka-connector/current/).

#### Migrations

The migration triggers are insertion of documents in the migrations collection.
Those triggers are handled by the [clinic-worker](https://github.com/tidepool-org/clinic-worker) service, 
which migrates patient profiles from a legacy clinic account to a clinic in the clinic service.

#### User deletions

Every time a user is deleted from the system, the clinic-worker service deletes the corresponding patient
and clinician records.

#### Emails

We send transaction emails to users when:
- a new clinic is created
- all patients of a clinic are migrated from a legacy account
- a patient sends an invite to a clinic
- an administrator changes the permissions of a clinician
- a clinic member creates a custodial account 

### Code generation

The OpenAPI Client and Server Code Generation tool generates the necessary files from
a bundled yaml file exported from Spotlight Studio. Unfortunately, Spotlight Studio 
exports the yaml file in a slightly different format than what the code generation tool
can import. We use [swagger-cli](https://github.com/APIDevTools/swagger-cli) for bundling
references.

### Development

You need to download and install `swagger-cli` and `oapi-codegen` binaries to be able to regenerate the code:
```
npm install -g @apidevtools/swagger-cli
go get github.com/deepmap/oapi-codegen/cmd/oapi-codegen
```

#### Makefile

The makefile has three targets:
* generate - code generation for server stubs, types and specification. Runs `swagger-cli` to bundle
refs. 
* test - runs test scripts
* build - builds package
