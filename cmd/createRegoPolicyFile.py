import json
import yaml

inFilename = "clinic.v1.yaml"
outFilename = "opa/clinic_policy.rego"

def createPolicy(operations, path, roles):
    segments = []
    hasClinicsParam = False
    for segment in path[1:].split("/"):
        if segment.startswith("{"):
            segments.append(segment.strip("{}"))
            if segment == "{clinicid}":
                hasClinicsParam = True
        else:
            segments.append('"{segment}"'.format(segment=segment))

    policyName = "_".join(segments)
    if len(roles) > 1:
        #rolesStr = ",".join(["{%s}" % role for role in roles])
        rolesStr = "|".join(roles)
    else:
        rolesStr = "".join(roles)
    rolesLine = ",".join(['"{role}"'.format(role=role) for role in roles])



    methodLine = ",".join(['input.method == "{operation}"'.format(operation=operation.upper()) for operation in operations])
    pathLine = ",".join(segments)
    policy = [
        "allow {",
        "  any([{methodLine}])".format(methodLine=methodLine),
        "  input.parsed_path = [{pathLine}]".format(pathLine=pathLine),
        ""
    ]
    if rolesLine:
        if hasClinicsParam:
            get_role = [
                '  # Get roles',
                '  url := sprintf("http://%s:%s/clinics/%s/clinicians/%s", [clinicService, clinicServicePort, clinicid, input.user_id])',
                '  response := http.send({',
                '    "headers": {"X-TIDEPOOL-USERID": "ADMIN"},',
                '    "method" : "GET",',
                '    "url": url',
                '  })',
                '',
                '  # Get input roles from response',
                '  input_roles := {y | y = response.body.permissions[_]} | {x | x = input.roles[_]}',
            ]
            policy.extend(get_role)
        else:
            get_role = [
                '  # Get roles',
                '  input_roles := {x | x = input.roles[_]}'
            ]
            policy.extend(get_role)

        policy.append("  roles := {{{rolesLine}}}".format(rolesLine=rolesLine))
        policy.append("  s := roles & input_roles")
        policy.append("")
        policy.append("  # Make sure valid role exists")
        policy.append("  count(s) > 0")

    policy.append("}")

    policy = [line + "\n" for line in policy]



    # policy = {
    #     "id": id,
    #     "subjects": ["users:*"],
    #     "resources": ["resources:%s" % resourceName],
    #     "actions": operations,
    #     "effect": "allow",
    #     "conditions": {
    #         "role": {
    #             "type": "StringMatchCondition",
    #             "options": {
    #                 "matches": rolesStr
    #             }
    #         },
    #     }
    # }
    #
    # # if we have an endpoint with clinicId - add string pairs
    # if "clinicid" in path:
    #     policy["conditions"]["clinics"] = {
    #             "type": "StringPairsEqualCondition",
    #             "options": {}
    #         }

    return policy


def main():
    validOps = ["get", "post", "patch", "delete"]
    # Read in clinic file
    with open(inFilename) as file:
        schema = yaml.load(file, Loader=yaml.FullLoader)

    # Iterate through paths
    allSummaries = []
    for path, endpoint in schema["paths"].items():
        # create policy for each path
        print(" ")
        summaryRecs = []
        for operation, value in endpoint.items():
            if (operation in validOps):
                print(operation, path, value["x-roles"])
                if len(summaryRecs) == 0:
                    summaryRecs.append({"operations": [operation], "path": path, "roles": value["x-roles"]})
                else:
                    for rec in summaryRecs:
                        if set(rec["roles"]) == set(value["x-roles"]):
                            rec["operations"].append(operation)
                            break
                    else:
                        summaryRecs.append({"operations": [operation], "path": path, "roles": value["x-roles"]})
        allSummaries.extend(summaryRecs)

    policies = []
    for rec in allSummaries:
        policies.append(createPolicy(rec["operations"], rec["path"], rec["roles"]))
        print(rec)

    # Write out system file
    with open(outFilename, 'w') as outfile:
        outfile.write("package clinics\n\n")
        outfile.write("default allow = false\n\n")
        outfile.write("clinicService = \"clinic\"\n")
        outfile.write("clinicServicePort = \"8080\"\n")
        outfile.write("\n\n")
        for policy in policies:
            outfile.writelines(policy)
            outfile.write("\n")
        #json.dump(policies, outfile, indent=4)

if __name__ == "__main__":
    main()


