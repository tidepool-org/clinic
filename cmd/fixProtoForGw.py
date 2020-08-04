import re


filename = 'generated/services/default_service.proto'
annotationImport = """

import "google/api/annotations.proto";
"""

output = []
with open(filename) as fp:
    lines = fp.readlines()
    for line in lines:
        if line.startswith("package"):
            output.append(line)
            output.append(annotationImport)
        elif line.strip().startswith('rpc'):

            name = line.strip().split(' ')[1]
            nameArray = re.findall('[A-Z][^A-Z]*', name)
            methodArray = []
            for part in nameArray[1:]:
                if part.endswith("id"):
                    methodArray.append("{{{part}}}".format(part=part.lower()))
                else:
                    methodArray.append(part.lower())
            outline = """
    {line} {{
            option (google.api.http) = {{
                {op}: "/{method}"
        }};
    }}""".format(line=line.strip(" ;\n"), op=nameArray[0].lower(), method="/".join(methodArray))
            output.append(outline)
        else:
            output.append(line)

with open(filename, "w") as fp:
    fp.writelines(output)
