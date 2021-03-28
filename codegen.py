from qface.generator import FileSystem, Generator
import logging.config
import argparse
from path import Path
import qface
import subprocess
import sys
import os

parser = argparse.ArgumentParser(description='Generate high-level IPC/RPC interfaces defined in qface based on godbus')
parser.add_argument('--src', dest='src', type=str, required=False, default='.',
                    help='where all .qface definitions are located (possibly in sub-directories), default value is current directory')
parser.add_argument('--input', dest='input', type=str, required=True, nargs='+',
                    help='qface interface relative to src path')
parser.add_argument('--output', dest='output', type=str, required=False, default='.',
                    help='path to place the generated code relative to go module base path, default value is current directory')
args = parser.parse_args()


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


def insert_unique_dependency_module(symbol, dependencies):
    deducted_symbol = symbol.type.nested if symbol.type.nested else symbol
    if not deducted_symbol.type.is_primitive:
        found_symbol = module.lookup(deducted_symbol.type.qualified_name)
        if found_symbol:
            dependency = found_symbol.module
            if dependency and dependency.module != deducted_symbol.type.module and dependency not in dependencies:
                dependencies.append(dependency)


def interface_dependencies(self):
    dependencies = []
    for prop in self.properties:
        insert_unique_dependency_module(prop, dependencies)
    for operation in self.operations:
        for param in operation.parameters:
            insert_unique_dependency_module(param, dependencies)
        if operation.has_return_value:
            insert_unique_dependency_module(operation.type, dependencies)
    for m in self.signals:
        for param in m.parameters:
            insert_unique_dependency_module(param, dependencies)
    return dependencies


def interface_imports(self):
    imports = {}
    for interface in self.interfaces:
        for dependency in interface_dependencies(interface):
            imports[''.join(dependency.name_parts)] = dependency.tags.get('gomod')
    return imports


def struct_imports(self):
    imports = {}
    dependencies = []
    for struct in self.structs:
        for field in struct.fields:
            insert_unique_dependency_module(field, dependencies)
    for dependency in dependencies:
        imports[''.join(dependency.name_parts)] = dependency.tags.get('gomod')
    return imports


FileSystem.strict = True
Generator.strict = True

setattr(qface.idl.domain.Module, 'interface_imports', property(interface_imports))
setattr(qface.idl.domain.Module, 'struct_imports', property(struct_imports))

setattr(qface.idl.domain.TypeSymbol, 'go_type', property(go_type))
setattr(qface.idl.domain.Field, 'go_type', property(go_type))
setattr(qface.idl.domain.Operation, 'go_type', property(go_type))
setattr(qface.idl.domain.Property, 'go_type', property(go_type))
setattr(qface.idl.domain.Parameter, 'go_type', property(go_type))

setattr(qface.idl.domain.Field, 'cap_name', property(cap_name))

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


here = Path(__file__).dirname()
inputs = []
for i in args.input:
    inputs.append(os.path.join(args.src, i))
system = FileSystem.parse(inputs)
modulesToGenerate = [module.name for module in system.modules]
system = FileSystem.parse(args.src)
output = args.output
generator = Generator(search_path=Path(here / 'templates'))
generator.destination = output
ctx = {'output': output}

for module in system.modules:
    if module.name in modulesToGenerate:
        ctx.update({'module': module})
        module_path = '/'.join(module.name_parts)
        ctx.update({'path': module_path})
        generator.write('{{path}}/' + module.name_parts[-1] + 'Interface.go', 'Interface.go.template', ctx)
        generator.write('{{path}}/' + module.name_parts[-1] + 'DBusAdapter.go', 'DBusAdapter.go.template', ctx)
        generator.write('{{path}}/' + module.name_parts[-1] + 'Enums.go', 'Enum.go.template', ctx)
        generator.write('{{path}}/' + module.name_parts[-1] + 'Structs.go', 'Struct.go.template', ctx)
        generator.write('{{path}}/' + module.name_parts[-1] + 'DBusProxy.go', 'DBusProxy.go.template', ctx)
