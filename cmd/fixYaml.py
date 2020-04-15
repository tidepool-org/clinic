import yaml
from urllib.parse import unquote
import sys

def findKey(d, key):
    if isinstance(d, dict):
        if key in d:
            yield d
        for k in d:
            yield from findKey(d[k], key)

def findSchema(d, path):
    for p in path.split("/")[1:-1]:
        d = d[p.replace("~1", "/")]
    return d

if __name__ == "__main__":
    key = "$ref"
    with open(r'clinic.v1.yaml') as file:
        schema = yaml.load(file, Loader=yaml.FullLoader)

        # Get list of refs
        refs = list(findKey(schema, key))

        # Get distinct list of schema paths
        schemaPaths = []
        for ref in refs:
            if unquote(ref[key]) not in schemaPaths:
                schemaPaths.append(unquote(ref[key]))

        # Move ref to components section
        for path in schemaPaths:
            schemaSection = findSchema(schema, path)
            title = schemaSection["schema"]['title']
            schema["components"]["schemas"][title] = schemaSection["schema"]
            newPath = "#/components/schemas/%s" % title
            schemaSection["schema"] = {"$ref": newPath}

            # update all refs for this path
            for ref in refs:
                if unquote(ref[key]) == path:
                    ref[key] = newPath

        with open(r'clinic.bundled.y1.yaml', 'w') as outfile:
            documents = yaml.dump(schema, outfile)
