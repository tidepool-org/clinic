import random
import faker
import json
import requests
from enum import Enum
import copy
import csv
import sys
import clinicTestData

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

class Strategy(Enum):
    RandomStrategy = 1
    FixedStrategy = 2
OperationStrategy = Strategy.FixedStrategy

PopulateSequence = [0,0,0,0,5,5,5,5,5,5,5,5,5,5,5,5,5,10,10,10,10,10,10,10,10,10,10,10,10,10]
FixedStrategySequence = [1,2,3,2,0,4,5,5,5,5,6,7,5,5,8,9,11,11,12,14,13,13,14]

def getNextOp():
    getNextOp.CurOperationIndex += 1

    # First populate with populationSequence
    if getNextOp.CurOperationIndex < len(PopulateSequence):
        return Operations[PopulateSequence[getNextOp.CurOperationIndex]]

    # Use random strategy if user specified
    if OperationStrategy == Strategy.RandomStrategy:
        return random.choice(Operations)

    # Use fixed strategy - account for initial sequence
    if OperationStrategy == Strategy.FixedStrategy:
        index = getNextOp.CurOperationIndex - len(PopulateSequence)
        if index < len(FixedStrategySequence):
            return Operations[FixedStrategySequence[index]]
        else:
            return Operations[index % len(Operations)]
getNextOp.CurOperationIndex = -1

envs = {
    'int': 'https://external.integration.tidepool.org',
    'prd': 'https://api.tidepool.org',
    'qa1': 'https://qa1.development.tidepool.org',
    'qa2': 'https://qa2.development.tidepool.org',
    'dev': 'https://dev1.dev.tidepool.org',
    'local': 'http://localhost:{}'.format(LocalPort)
}
environment = 'local'
environment = 'dev'
AuthUrl = '/auth/login'


def createRandomClinicAddBody(paramValues):
    name = random.choice(ClinicNames)
    location = random.choice(Locations) if name in UsedClinics else ""
    UsedClinics.append(name)
    clinic = {
        "address": fake.address(),
        "name": name
    }
    if location:
        clinic["location"] = location
    return json.dumps(clinic)



def createRandomClinicModifyBody(paramValues):
    clinic = {
        "Address": fake.address(),
    }
    return json.dumps(clinic)

Permissions = ["CLINIC_ADMIN", "CLINIC_CLINICIAN", "CLINIC_PRESCRIBER"]
PatientPermissions = ["READ", "WRITE"]
def createRandomClinicianAddBody(paramValues):
    clinicsClinians = {
        "clinicId": paramValues["clinicid"],
        "clinicianId": paramValues["clinicianid"],
        "permissions": [random.choice(Permissions)],
    }
    return json.dumps(clinicsClinians)

def createRandomClinicianModifyBody(paramValues):
    return createRandomClinicianAddBody(paramValues)

def createRandomPatientAddBody(paramValues):
    clinicsPatients = {
        "clinicId": paramValues["clinicid"],
        "patientId": paramValues["patientid"],
        "permissions": [random.choice(PatientPermissions)],
    }
    return json.dumps(clinicsPatients)

def createRandomPatientModifyBody(paramValues):
    return createRandomPatientAddBody(paramValues)


Operations = [
    {"name": "Add Clinic", "op": "POST", "path": "/clinics", "body": createRandomClinicAddBody},
    {"name": "Get Clinics", "op": "GET", "path": "/clinics", "roles": "TIDEPOOL_ADMIN"},
    {"name": "Get Clinic", "op": "GET", "path": "/clinics/{clinicid}", "params": ["clinicid"]},
    {"name": "Modify Clinic", "op": "PATCH", "path": "/clinics/{clinicid}", "params": ["clinicid"], "body": createRandomClinicModifyBody},
    {"name": "Remove Clinic", "op": "DELETE", "path": "/clinics/{clinicid}", "params": ["clinicid"]},

    {"name": "Add Clinician", "op": "POST", "path": "/clinics/{clinicid}/clinicians", "body": createRandomClinicianAddBody, "params": ["clinicid"], "randomid": "clinicianid"},
    {"name": "Get Clinicians", "op": "GET", "path": "/clinics/{clinicid}/clinicians", "params": ["clinicid"]},
    {"name": "Get Clinician", "op": "GET", "path": "/clinics/{clinicid}/clinicians/{clinicianid}", "params": ["clinicid", "clinicianid"]},
    {"name": "Modify Clinician", "op": "PATCH", "path": "/clinics/{clinicid}/clinicians/{clinicianid}", "params": ["clinicid", "clinicianid"], "body": createRandomClinicianModifyBody},
    {"name": "Remove Clinician", "op": "DELETE", "path": "/clinics/{clinicid}/clinicians/{clinicianid}", "params": ["clinicid", "clinicianid"]},

    {"name": "Add Patient", "op": "POST", "path": "/clinics/{clinicid}/patients", "body": createRandomPatientAddBody, "params": ["clinicid"], "randomid": "patientid"},
    {"name": "Get Patients", "op": "GET", "path": "/clinics/{clinicid}/patients", "params": ["clinicid"]},
    {"name": "Get Patient", "op": "GET", "path": "/clinics/{clinicid}/patients/{patientid}", "params": ["clinicid", "patientid"]},
    {"name": "Modify Patient", "op": "PATCH", "path": "/clinics/{clinicid}/patients/{patientid}", "params": ["clinicid", "patientid"], "body": createRandomPatientModifyBody},
    {"name": "Remove Patient", "op": "DELETE", "path": "/clinics/{clinicid}/patients/{patientid}", "params": ["clinicid", "patientid"]},
]



MinRemoveCount = 4
NumberOps = 40
CredentialFile = "ClinicTest-Credentials.csv"
CredentialTable = []
CredentialMap = {}

def validOperation(rec, clinicList):
    if rec["name"] == "Add Clinic":
        # Special remove case
        if rec["op"] == "DELETE" and len(clinicList) < MinRemoveCount:
            return False

    return True

def randomId():
    if randomId.index == 0:
        # Read credential file
        with open(CredentialFile) as csvfile:
            reader = csv.DictReader(csvfile, fieldnames=["name", "url", "username", "password", "recname"])
            for row in reader:
                CredentialTable.append({"username": row["username"], "password": row["password"]})
    authRec = CredentialTable[randomId.index]
    if environment != "local":
        req = requests.post(getFullPath(AuthUrl), auth=(authRec["username"], authRec["password"]))
        if req.status_code == 200:
            authRec['token'] = req.headers['x-tidepool-session-token']
            userid = req.json()['userid']
            authRec['userid'] = userid
            CredentialMap[userid] = authRec
        else:
            print("Could not log user: {user} in - status: {status}".format(user=authRec["username"], status=req.status_code))
            sys.exit()
    else:
        userid = ''.join(random.choice('0123456789abcdef') for x in range(0,16))
        authRec['userid'] = userid
    randomId.index += 1
    return userid

randomId.index = 0

def getFullPath(path):
    return "{prefix}{path}".format(prefix=envs[environment], path=path)

def getParamValues(rec, clinicList, clinicianMap, patientMap):
    params = {}
    if "params" in rec:
        if "clinicid" in rec["params"]:
            if len(clinicList) > 0:
                clinicId = random.choice(clinicList)
            else:
                return None

            params["clinicid"] = clinicId

            if "clinicianid" in rec["params"]:
                if clinicId in clinicianMap and len(clinicianMap[clinicId]) > 1:
                    params["clinicianid"] = random.choice(clinicianMap[clinicId][1:])
                else:
                    return None

            if "patientid" in rec["params"]:
                if clinicId in patientMap and len(patientMap[clinicId]) > 0:
                    params["patientid"] = random.choice(patientMap[clinicId])
                else:
                    return None

    # fill out ids used for posts patient and clinician
    if "randomid"  in rec:
        params[rec["randomid"]] = randomId()
    return params


def executeOperation(rec, paramValues, clinicianMap, patientMap):
    data = None
    # If adding clinic - use random id
    if rec["name"] == "Add Clinic" or rec["name"] == "Get Clinics":
        rec["userid"] = randomId()

    # If a clinician - use admin
    elif "Clinician" in rec["name"]:
        clinicid = paramValues["clinicid"]
        rec["userid"] = clinicianMap[clinicid][0]

    # If a parient - use any doc
    elif "Clinician" in rec["name"]:
        clinicid = paramValues["clinicid"]
        rec["userid"] = clinicianMap[clinicid].randomChoice()

    # Else - it is a clinic - use admin
    else:
        clinicid = paramValues["clinicid"]
        rec["userid"] = clinicianMap[clinicid][0]




    if environment != "local":
        headers = {
            "x-tidepool-session-token": CredentialMap[rec["userid"]]["token"],
            "content-type" :"application/json"
        }

    else:
        headers = {
            "X-TIDEPOOL-USERID": rec["userid"],
            "content-type" :"application/json"
        }
        if "roles" in rec:
            headers["X-TIDEPOOL-ROLES"] = rec["roles"]

    if "body" in rec:
        data = rec["body"](paramValues)
    if rec["op"] == "GET":
        r = requests.get(getFullPath(rec["path"]),data=data, headers=headers)
    elif rec["op"] == "POST":
        print("Calling: {path}  -- pvs: {pvs}".format(path=getFullPath(rec["path"]),  pvs=paramValues))
        r = requests.post(getFullPath(rec["path"]),data=data, headers=headers)
        print("Received status code: " + str(r.headers))
        ret = r.json()
        if "id" in ret:
            rec["id"] = ret["id"]
    elif rec["op"] == "PATCH":
        r = requests.patch(getFullPath(rec["path"]),data=data, headers=headers)
    elif rec["op"] == "DELETE":
        r = requests.delete(getFullPath(rec["path"]),data=data, headers=headers)
    else:
        print("Unkown op:, {}", rec["op"])
        return False

    print("Called: {path}  -- userid: {userid}  return code: {status_code}, pvs: {pvs}".format(path=rec["path"], userid=rec["userid"], status_code=r.status_code, pvs=paramValues))
    if "id" in rec:
        print("Rec id: {id}".format(id=rec["id"]))

    if r.status_code != 200:
        return False
    return True

def updateInternalTables(rec, clinicList, paramValues, clinicianMap, patientMap):
    # if add - place in correct record
    if rec["name"] == "Add Clinic":
        clinicList.append(rec["id"])
        if rec["id"] not in clinicianMap:
            clinicianMap[rec["id"]] = []
        clinicianMap[rec["id"]].append(rec["userid"])

    if rec["name"] == "Remove Clinic":
        clinicList.remove(paramValues["clinicid"])
        del clinicianMap[paramValues["clinicid"]]

    if rec["name"] == "Add Clinician":
        # For a clinician - we must add new clinician to
        clinicianMap[paramValues["clinicid"]].append(paramValues["clinicianid"])

    if rec["name"] == "Remove Clinician":
        clinicianMap[paramValues["clinicid"]].remove(paramValues["clinicianid"])

    if rec["name"] == "Add Patient":
        # For a clinician - we must add new clinician to
        if paramValues["clinicid"] not in patientMap:
            patientMap[paramValues["clinicid"]] = []
        patientMap[paramValues["clinicid"]].append(paramValues["patientid"])

    if rec["name"] == "Remove Patient":
        patientMap[paramValues["clinicid"]].remove(paramValues["patientid"])




def main():
    # Just loop through doing operations

    clinicList = []
    clinicianMap = {}
    patientMap = {}
    for opCount in range(1,NumberOps):
        # First - get an operation
        op = getNextOp()
        rec = copy.deepcopy(op)

        # Get all parameters for the operation
        paramValues = getParamValues(rec, clinicList, clinicianMap, patientMap)
        if paramValues == None:
            print("Could not find parameters for this operation")
            continue

        # Fill out path
        rec["path"] = rec["path"].format(**paramValues)
        print("Op: {op}, Path: {path}".format(op=rec["op"], path=rec["path"]))

        # Make sure it is a valid operation
        if not validOperation(rec, clinicList):
            print("Not a valid operation")
            continue

        # execute operation
        if not executeOperation(rec, paramValues, clinicianMap, patientMap):
            print("Operation Failed - quitting")
            continue

        # update our internal Tables
        updateInternalTables(rec, clinicList, paramValues, clinicianMap, patientMap)


def createRandomClinics():
    paramValues = {}
    names = []
    for i in range(1,20):
        clinic = json.loads(createRandomClinicAddBody(paramValues))
        if clinic["name"] in names:
            continue
        names.append(clinic["name"])
        print (clinic)


def createRandomPatients():
    index = 0
    for id in clinicTestData.clinics:
        numPatients = random.randint(1,50)
        for patientid in clinicTestData.patients[index:index+numPatients]:
            print(createRandomPatientAddBody({"clinicid": id, "patientid": patientid}))
        index += numPatients

def createRandomClinicians():
    index = 0
    for id in clinicTestData.clinics:
        numClinicians = random.randint(2,9)
        for clinicianid in clinicTestData.clinicians[index:index+numClinicians]:
            print(createRandomClinicianAddBody({"clinicid": id, "clinicianid": clinicianid}))

if __name__ == "__main__":
    main()
    #createRandomClinicians()

