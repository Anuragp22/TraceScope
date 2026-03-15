; Function definitions
(function_definition
  name: (identifier) @func.name) @func.def

; Class definitions
(class_definition
  name: (identifier) @class.name) @class.def

; Call expressions - simple
(call
  function: (identifier) @call.name) @call.expr

; Call expressions - attribute (obj.method())
(call
  function: (attribute
    object: (identifier) @call.receiver
    attribute: (identifier) @call.attr_name)) @call.attr_expr

; Import statements
(import_statement
  name: (dotted_name) @import.name) @import.decl

; Import-from statements
(import_from_statement
  module_name: (dotted_name) @from.module) @from.decl

; Decorated definitions (to detect decorators but we capture the function)
(decorated_definition
  (function_definition
    name: (identifier) @decorated_func.name)) @decorated_func.def
