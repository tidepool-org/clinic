import yaml
from urllib.parse import unquote
import sys

inFilename = 'clinic.v1.yaml'
outFilename = 'clinic.fixed.v1.yaml'

def findKey(d, key):
    if isinstance(d, dict):
        if key in d:
            yield d
        for k in d:
            yield from findKey(d[k], key)
    if isinstance(d, list):
        for val in d:
            yield from findKey(val, key)

def findSchema(d, path):
    for p in path.split("/")[1:-1]:
        d = d[p.replace("~1", "/")]
    return d

if __name__ == "__main__":
    key = "$ref"
    with open(inFilename) as file:
        schema = yaml.load(file, Loader=yaml.FullLoader)

        # Get list of refs
        refs = list(findKey(schema, key))

        # Get distinct list of schema paths
        schemaPaths = []
        for ref in refs:
            print(ref)
            if unquote(ref[key]) not in schemaPaths:
                schemaPaths.append(unquote(ref[key]))

        # Move ref to components section
        for path in schemaPaths:
            schemaSection = findSchema(schema, path)
            print("schema section, path", schemaSection, path, "\n")
            title = schemaSection["schema"]['title']
            schema["components"]["schemas"][title] = schemaSection["schema"]
            newPath = "#/components/schemas/%s" % title

            # update all refs for this path
            for ref in refs:
                if unquote(ref[key]) == path:
                    ref[key] = newPath

            schemaSection["schema"] = {"$ref": newPath}

        with open(outFilename, 'w') as outfile:
            documents = yaml.dump(schema, outfile)
