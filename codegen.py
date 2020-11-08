from qface.generator import FileSystem, Generator
import logging.config
import argparse
from path import Path
import qface

parser = argparse.ArgumentParser(description='Generate golang godbus interface based on qface')
parser.add_argument('--input', dest='input', type=str, required=True,
                    help='input qface to generate godbus interface')
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
        return self.type


def has_return_value(self):
    return not self.type.name == 'void'


def cap_name(self):
    return ' '.join(word[0].upper() + word[1:] for word in self.name.split())


def lower_name(self):
    return ' '.join(word[0].lower() + word[1:] for word in self.name.split())


def export_name(self):
    return cap_name(self) + "Interface"


def proxy_name(self):
    return cap_name(self) + "Proxy"


def param_size(self):
    return len(self.parameters)


FileSystem.strict = True
Generator.strict = True

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
setattr(qface.idl.domain.Interface, 'export_name', property(export_name))
setattr(qface.idl.domain.Interface, 'proxy_name', property(proxy_name))

setattr(qface.idl.domain.Property, 'lower_name', property(lower_name))
setattr(qface.idl.domain.Property, 'cap_name', property(cap_name))

setattr(qface.idl.domain.Signal, 'cap_name', property(cap_name))
setattr(qface.idl.domain.Signal, 'param_size', property(param_size))


here = Path(__file__).dirname()
system = FileSystem.parse(args.input)
generator = Generator(search_path=Path(here / 'templates'))
for module in system.modules:
    ctx = {'module': module}
    module_path = '/'.join(module.name_parts)
    ctx.update({'path': module_path})
    generator.write('{{path}}/' + module.name_parts[-1] + 'DBusAdapter.go', 'DBusAdapter.go.template', ctx)
    generator.write('{{path}}/' + module.name_parts[-1] + 'Enums.go', 'Enum.go.template', ctx)
    generator.write('{{path}}/' + module.name_parts[-1] + 'Structs.go', 'Struct.go.template', ctx)
    generator.write('{{path}}/' + module.name_parts[-1] + 'DBusProxy.go', 'DBusProxy.go.template', ctx)
