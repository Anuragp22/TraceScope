; Function declarations
(function_declaration
  name: (identifier) @func.name) @func.def

; Method declarations
(method_declaration
  receiver: (parameter_list
    (parameter_declaration
      type: (_) @method.receiver_type))
  name: (field_identifier) @method.name) @method.def

; Call expressions
(call_expression
  function: (identifier) @call.name) @call.expr

; Selector call expressions (pkg.Func or obj.Method)
(call_expression
  function: (selector_expression
    operand: (identifier) @call.receiver
    field: (field_identifier) @call.selector_name)) @call.selector_expr

; Import declarations
(import_spec
  name: (package_identifier)? @import.alias
  path: (interpreted_string_literal) @import.path) @import.spec

; Package clause
(package_clause
  (package_identifier) @package.name)

; Struct type declarations
(type_declaration
  (type_spec
    name: (type_identifier) @struct.name
    type: (struct_type))) @struct.def

; Interface type declarations
(type_declaration
  (type_spec
    name: (type_identifier) @interface.name
    type: (interface_type))) @interface.def
