from qface.generator import FileSystem, Generator, System
import logging.config
import argparse
import json
from path import Path
import qface
import subprocess
import sys
import os

logger = logging.getLogger(__name__)

parser = argparse.ArgumentParser(description='Generates bindings for godbus based on the qface IDL.')
parser.add_argument('--dependency', dest='dependency', type=str, required=False, nargs='+', default=[],
                    help='path to dependency .qface files, leave empty if there is no interdependency')
parser.add_argument('--input', dest='input', type=str, required=True, nargs='+',
                    help='qface interface relative to src path')
parser.add_argument('--output', dest='output', type=str, required=False, default='.',
                    help='path to place the generated code relative to go module base path, default value is current directory')
args = parser.parse_args()

yaml_annotate = ".go.annotate"


def go_type(self: object) -> object:
    if self.type.is_primitive:
        if self.type.name == 'real':
            return 'float32'
        return self.type
    elif self.type.is_void:
        return "what"
    elif self.type.is_list:
        return '[]{0}'.format(go_type(self.type.nested))
    elif self.type.is_map:
        return 'map[string]{0}'.format(go_type(self.type.nested))
    else:
        split = self.type.name.split(".")
        if len(split) > 1:
            if self.type.name.rpartition('.')[0] == self.module.name:
                return split[-1]
            return ''.join(split[:-1]) + '.' + split[-1]
        else:
            return split[0]


def has_return_value(self):
    return not self.type.name == 'void'


def cap_name(self):
    return ' '.join(word[0].upper() + word[1:] for word in self.name.split())


def lower_name(self):
    return ' '.join(word[0].lower() + word[1:] for word in self.name.split())


def proxy_name(self):
    return cap_name(self) + "Proxy"


def param_size(self):
    return len(self.parameters)


def insert_unique_dependency_module(module, symbol, dependencies):
    deducted_symbol = symbol.type.nested if symbol.type.nested else symbol
    if not deducted_symbol.type.is_primitive:
        found_symbol = module.lookup(deducted_symbol.type.qualified_name)
        if found_symbol:
            dependency = found_symbol.module
            if dependency and dependency.module != deducted_symbol.type.module and dependency not in dependencies:
                dependencies.append(dependency)


def base_dependencies(module, interface):
    dependencies = []
    for prop in interface.properties:
        insert_unique_dependency_module(module, prop, dependencies)
    for m in interface.signals:
        for param in m.parameters:
            insert_unique_dependency_module(module, param, dependencies)
    return dependencies


def interface_dependencies(module, interface):
    dependencies = []
    for operation in interface.operations:
        for param in operation.parameters:
            insert_unique_dependency_module(module, param, dependencies)
        if operation.has_return_value:
            insert_unique_dependency_module(module, operation.type, dependencies)
    for base_dependency in base_dependencies(module, interface):
        dependencies.append(base_dependency)
    return dependencies


def base_imports(self):
    imports = {}
    for interface in self.interfaces:
        for dependency in base_dependencies(self, interface):
            imports[''.join(dependency.name_parts)] = dependency.tags.get('gomod')
    return imports


def interface_imports(self):
    imports = {}
    for interface in self.interfaces:
        for dependency in interface_dependencies(self, interface):
            imports[''.join(dependency.name_parts)] = dependency.tags.get('gomod')
    return imports


def struct_imports(self):
    imports = {}
    dependencies = []
    for struct in self.structs:
        for field in struct.fields:
            insert_unique_dependency_module(self, field, dependencies)
    for dependency in dependencies:
        imports[''.join(dependency.name_parts)] = dependency.tags.get('gomod')
    return imports


def unique_enum_name(self):
    module_members = []
    for enum in self.module.enums:
        for member in enum.members:
            if member == self:
                if member.name not in module_members:
                    return self.name
                else:
                    return self.enum.name + self.name
            else:
                module_members.append(member.name)
    raise Exception("This should logically never happen")


def get_go_mod_path(path: Path):
    go_mod = ""
    try:
        result = subprocess.run(['go mod edit -json'], stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, cwd=path, shell=True).stdout.decode('utf-8')
        if result != "":
            go_mod = json.loads(result)["Module"]["Path"]
    except KeyError as exc:
        message = 'Key error getting go mod: {0}'.format(exc)
        print(message)
        raise
    return go_mod


def construct_gomod_tag(module, output):
    module_path = '/'.join(module.name_parts)
    abs_path = os.path.abspath(output)
    current_path = abs_path
    go_mod = get_go_mod_path(current_path)
    while go_mod != "" and get_go_mod_path(os.path.dirname(current_path)) == go_mod:
        current_path = os.path.dirname(current_path)

    commonprefix = os.path.commonprefix([current_path, abs_path])
    return os.path.join(go_mod + abs_path.replace(commonprefix, ''), module_path)


def generate_annotate(input, output: str):
    inputs = input if isinstance(input, (list, tuple)) else [input]
    for input in inputs:
        path = Path.getcwd() / str(input)
        if path.isfile():
            write_annotate(path, output)
        else:
            for document in path.walkfiles("*.qface"):
                write_annotate(document, output)


def write_annotate(document: Path, output: str):
    system = FileSystem.parse(document)
    f = open(document.stripext() + yaml_annotate, "w")
    for module in system.modules:
        f.write(module.name + ":\n   gomod:\n    \"" + construct_gomod_tag(module, output) + "\"")
    f.close()


def merge_generated_annotation(input, system: System):
    inputs = input if isinstance(input, (list, tuple)) else [input]
    for input in inputs:
        path = Path.getcwd() / str(input)
        if path.isfile():
            FileSystem.merge_annotations(system, path.stripext() + yaml_annotate)
        else:
            for document in path.walkfiles("*.qface"):
                FileSystem.merge_annotations(system, document.stripext() + yaml_annotate)


FileSystem.strict = True
Generator.strict = True

setattr(qface.idl.domain.Module, 'interface_imports', property(interface_imports))
setattr(qface.idl.domain.Module, 'base_imports', property(base_imports))
setattr(qface.idl.domain.Module, 'struct_imports', property(struct_imports))

setattr(qface.idl.domain.TypeSymbol, 'go_type', property(go_type))
setattr(qface.idl.domain.Field, 'go_type', property(go_type))
setattr(qface.idl.domain.Operation, 'go_type', property(go_type))
setattr(qface.idl.domain.Property, 'go_type', property(go_type))
setattr(qface.idl.domain.Parameter, 'go_type', property(go_type))

setattr(qface.idl.domain.Field, 'cap_name', property(cap_name))

setattr(qface.idl.domain.EnumMember, 'unique_name', property(unique_enum_name))

setattr(qface.idl.domain.Operation, 'has_return_value', property(has_return_value))
setattr(qface.idl.domain.Operation, 'cap_name', property(cap_name))
setattr(qface.idl.domain.Operation, 'lower_name', property(lower_name))

setattr(qface.idl.domain.Interface, 'cap_name', property(cap_name))
setattr(qface.idl.domain.Interface, 'lower_name', property(lower_name))
setattr(qface.idl.domain.Interface, 'proxy_name', property(proxy_name))

setattr(qface.idl.domain.Property, 'lower_name', property(lower_name))
setattr(qface.idl.domain.Property, 'cap_name', property(cap_name))

setattr(qface.idl.domain.Signal, 'cap_name', property(cap_name))
setattr(qface.idl.domain.Signal, 'lower_name', property(lower_name))
setattr(qface.idl.domain.Signal, 'param_size', property(param_size))


def generate():
    here = Path(__file__).dirname()
    inputs = []
    for i in args.input:
        inputs.append(i)
    generate_annotate(inputs, args.output)
    system = FileSystem.parse(inputs)
    module_to_generate = [module.name for module in system.modules]
    system = FileSystem.parse(inputs + args.dependency)
    merge_generated_annotation(inputs + args.dependency, system)
    output = args.output
    generator = Generator(search_path=Path(here / 'templates'))
    generator.destination = output
    ctx = {'output': output}

    for module in system.modules:
        if module.name in module_to_generate:
            ctx.update({'module': module})
            module_path = '/'.join(module.name_parts)
            ctx.update({'path': module_path})
            if module.interfaces:
                generator.write('{{path}}/' + module.name_parts[-1] + 'Interface.go', 'Interface.go.template', ctx)
                generator.write('{{path}}/' + module.name_parts[-1] + 'Base.go', 'Base.go.template', ctx)
                generator.write('{{path}}/' + module.name_parts[-1] + 'DBusAdapter.go', 'DBusAdapter.go.template', ctx)
                generator.write('{{path}}/' + module.name_parts[-1] + 'DBusProxy.go', 'DBusProxy.go.template', ctx)
            if module.enums:
                generator.write('{{path}}/' + module.name_parts[-1] + 'Enums.go', 'Enum.go.template', ctx)
            if module.structs:
                generator.write('{{path}}/' + module.name_parts[-1] + 'Structs.go', 'Struct.go.template', ctx)


if __name__ == '__main__':
    generate()
