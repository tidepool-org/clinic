import json
import yaml

inFilename = "clinic.v1.yaml"
outFilename = "keto/clinic_policy.json"

def createPolicy(operations, path, roles):
    resourceNames = []
    segments = []
    for segment in path[1:].split("/"):
        if segment.startswith("{"):
            segments.append('id')
            resourceNames.append('*')
        else:
            segments.append(segment)
            resourceNames.append(segment)

    id = "_".join(operations + segments)
    resourceName = ":".join(resourceNames)
    if len(roles) > 1:
        #rolesStr = ",".join(["{%s}" % role for role in roles])
        rolesStr = "|".join(roles)
    else:
        rolesStr = "".join(roles)

    policy = {
        "id": id,
        "subjects": ["users:*"],
        "resources": ["resources:%s" % resourceName],
        "actions": operations,
        "effect": "allow",
        "conditions": {
            "role": {
                "type": "StringMatchCondition",
                "options": {
                    "matches": rolesStr
                }
            },
        }
    }

    # if we have an endpoint with clinicId - add string pairs
    if "clinicid" in path:
        policy["conditions"]["clinics"] = {
                "type": "StringPairsEqualCondition",
                "options": {}
            }

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
        json.dump(policies, outfile, indent=4)

if __name__ == "__main__":
    main()


