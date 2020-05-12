import random
import faker
import json
import requests

ClinicNames = [
    "Patient’s Choice Medical Clinic",
    "Dignity Health",
    "Canyon Medical Center",
    "Desert Inn Medical Center",
    "Women’s Resource Clinic",
    "Healthcare",
    "Visiting Medical Clinic",
    "Oriental Medical Clinic",
    "Valley clinic",
    "Miracle clinic",
    "The Hope Clinic",
    "Tricare Medical Center",
    "Clinica",
    "MinuteClinic",
    "First Person Care Clinic",
    "A-Z Women’s Center",
    "Urgent Care",
    "Cleveland Clinic",
    "Modern Wellness Clinic",
    "DeNovo Clinic",
    "Pain Management",
    "Treatment Solutions",
    "TeleMind Clinic",
    "CareNow",
    "Men’s Focus",
    "helping kids clinic",
    "Mission Treatment",
    "FirstMed Clinic",
    "Healthy Minds",
    "Quick Care",
    "Hanger Clinic",
    "Union Family Health Center",
    "Perfect 32 Dental Care",
    "MyMedical",
    "Charter Medical",
    "The Bodywise Clinic",
    "Medicus Medical Centre",
    "Vista Clinic",
    "Optilase Clinic",
    "The Meath Clinic",
]

Locations = ["Main", "Satellite", "Branch"]
UsedClinics = []
fake = faker.Faker()

LocalPort = 8080

envs = {
    'int': 'https://external.integration.tidepool.org',
    'prd': 'https://api.tidepool.org',
    'qa1': 'https://qa1.development.tidepool.org',
    'qa2': 'https://qa2.development.tidepool.org',
    'local': 'http://localhost:{}'.format(LocalPort)
}
environment = 'local'


def createRandomClinicAddBody():
    name = random.choice(ClinicNames)
    location = name in UsedClinics if random.choice(Locations) else ""
    UsedClinics.append(name)
    clinic = {
        "address": fake.address(),
        "name": name
    }
    if location:
        clinic["location"] = location
    return json.dumps(clinic)



def createRandomClinicModifyBody():
    clinic = {
        "Address": fake.address(),
    }
    return json.dumps(clinic)



Operations = {
    "Add Clinic": {"op": "POST", "path": "/clinics", "body": createRandomClinicAddBody, "roles": "TIDEPOOL_ADMIN"},
    "Get Clinics": {"op": "GET", "path": "/clinics"},
    "Get Clinic": {"op": "GET", "path": "/clinics/{clinicid}", "params": ["clinicid"]},
    "Modify Clinic": {"op": "PATCH", "path": "/clinics/{clinicid}", "params": ["clinicid"], "body": createRandomClinicModifyBody()},
    "Remove Clinic": {"op": "DELETE", "path": "/clinics/{clinicid}", "params": ["clinicid"]},
}

MinRemoveCount = 4
NumberOps = 30

def validOperation(rec, paramMap):
    if "params" in rec:
        for param in rec["params"]:
            ids = paramMap[param]

            # No Parameter
            if len(ids) == 0:
                return False

            # Special remove case
            if rec["op"] == "DELETE" and len(ids) < MinRemoveCount:
                return False

    return True

def getParamValues(rec, paramMap):
    params = {}
    if "params" in rec:
        for param in rec["params"]:
            ids = paramMap[param]
            params[param] = random.choice(ids)
    return params

def randomId():
    return ''.join(random.choice('0123456789abcdef') for x in range(0,16))

def updateParamMap(rec, paramMap, paramValues, clinicianMap):
    # if add - place in correct record
    if rec["op"] == 'POST':
        if "clinicid" in paramValues:
            paramMap["clinicianid"].append(rec["id"])
        else:
            paramMap["clinicid"].append(rec["id"])
            clinicianMap.append(rec["userid"])

    if rec["op"] == 'DELETE':
        if "clinicianid" in paramValues:
            paramMap["clinicianid"].remove(paramValues["clinicianid"])
        else:
            paramMap["clinicid"].remove(paramValues["clinicid"])

def getFullPath(path):
    return "{prefix}{path}".format(prefix=envs[environment], path=path)

def executeOperation(rec, paramValues, clinicianMap):
    data = None
    if "params" in rec and "clinicid" in rec["params"]:
        clinicid = paramValues["clinicid"]
        rec["userid"] = random.choice(clinicianMap[clinicid])
    else:
        rec["userid"] = randomId()
    headers = {
        "X_TIDEPOOL_USERID": rec["userid"],
        "content-type" :"application/json"
    }
    if "roles" in rec:
        headers["X_TIDEPOOL_ROLES"] = rec["roles"]

    if "body" in rec:
        data = rec["body"]()
    if rec["op"] == "GET":
        r = requests.post(getFullPath(rec["path"]),data=data, headers=headers)
    elif rec["op"] == "POST":
        userid = randomId()
        r = requests.post(getFullPath(rec["path"]),data=data, headers=headers)
        ret = r.json()
        if "id" in ret:
            rec["id"] = ret["id"]
    elif rec["op"] == "PATCH":
        r = requests.post(getFullPath(rec["path"]),data=data, headers=headers)
    elif rec["op"] == "DELETE":
        r = requests.post(getFullPath(rec["path"]),data=data, headers=headers)
    else:
        print("Unkown op:, {}", rec["op"])
        return False

    print("Called: {path}  -- return code: {status_code}".format(path=rec["path"], status_code=r.status_code))
    if r.status_code != '200':
        return False
    return True

def main():
    # Just loop through doing operations

    paramMap = {
        "clinicid": [],
        "clinicianid": []
    }
    clinicianMap = {}
    for opCount in range(1,NumberOps):
        # Pick an operation
        while True:
            op = random.choice(list(Operations))
            rec = Operations[op]
            if validOperation(rec, paramMap):
                break

        # complete it
        paramValues = getParamValues(rec, paramMap)
        path = rec["path"].format(**paramValues)
        print("Op: {op}, Path: {path}".format(op=rec["op"], path=path))

        # execute operation
        if not executeOperation(rec, paramValues, clinicianMap):
            print("Operation Failed - quitting")
            continue

        # update info
        updateParamMap(rec, paramMap, paramValues, clinicianMap)

if __name__ == "__main__":
    main()