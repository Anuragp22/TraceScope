; Function declarations
(function_declaration
  name: (identifier) @func.name) @func.def

; Arrow functions assigned to variables
(lexical_declaration
  (variable_declarator
    name: (identifier) @arrow.name
    value: (arrow_function))) @arrow.def

; Variable-assigned functions
(lexical_declaration
  (variable_declarator
    name: (identifier) @varfunc.name
    value: (function_expression))) @varfunc.def

; Class declarations
(class_declaration
  name: (identifier) @class.name) @class.def

; Method definitions in classes
(method_definition
  name: (property_identifier) @method.name) @method.def

; Call expressions - simple
(call_expression
  function: (identifier) @call.name) @call.expr

; Call expressions - member (obj.method())
(call_expression
  function: (member_expression
    object: (identifier) @call.receiver
    property: (property_identifier) @call.member_name)) @call.member_expr

; Import declarations
(import_statement
  source: (string) @import.path) @import.decl

; Require calls
(call_expression
  function: (identifier) @_req (#eq? @_req "require")
  arguments: (arguments (string) @require.path)) @require.expr

; Export - named
(export_statement) @export.stmt
