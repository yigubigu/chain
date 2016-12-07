# Ivy Language Specification

## Lexical Elements

### Comment

```
line_comment      = "//" [^\n\r]*
multiline_comment = "/*" (!"*/" .)* "*/"
```

Line comments begin with `//` and stop at the end of the line.

Multiline comments begin with `/*` and stop with `*/`. Multiline comments must complete before the end of the file.

Comments are treated as a line break.

### Keywords

These keywords are reserved and may not be used as identifiers:

```
program
path
verify
satisfy
let
```

### Predefined Names

These identifiers are predefined as functions and may not be bound to values:

```
abs
min
max
within
sha256
sha3
checkSig
checkMultisig
```

These names are predefined as values and may not be reassigned:

```
tx
block
```

### Identifier

```
identifier = [A-Za-z_]+
```

Other than [keywords](#keywords) and [predefined names](#predefined-names), any sequence of uppercase and lowercase letters and/or underscores is considered an identifier.

### Delimiter

```
delimiter = "[" | "]" | "(" | ")" | "{" | "}" | ";" | "," | "."
```

#### Integer Literal

```
integer_literal = (-)?[0-9]+
```

A literal representing an [integer](#bytestring).

#### Hex Literal

```
hex_digit   = [0-9] | [a-f]
hex_literal = "0x" (hex_digit hex_digit)*
```

A literal representing a [bytestring](#bytestring).

For example, `0x` represents an empty bytestring, and `0xffff` represents a bytestring of two bytes, each of which would be represented as `11111111` in binary.

Hex literals must have an even number of nibs (i.e., `0xf` is not a valid bytestring literal).

### Boolean Literal

`boolean_literal = "true" | "false"`

A literal representing a [boolean](#boolean).

## Values

There are three types of values: bytestrings, integers, and booleans.

### Bytestring

A sequence of 0 or more bytes.

### Integer

An integer between -2^63 and 2^63 - 1, inclusive.

### Boolean

Either `true` or `false`.

## Expressions

Expressions _evaluate_ to values.

TBD: Precedence

### Literal Expression

`literal_expression = integer_literal | hex_literal | boolean_literal`

Literal expressions evaluate to the literal value they represent.

If an integer literal does not represent a valid integer value (i.e., the value is not within the range -2^63 and 2^63 - 1, inclusive), compilation fails with an error.

### Variable

```
variable = identifier
```

A variable evaluates to the value to which its identifier is bound in the current scope.

If the identifier is not bound to any value in the current scope, compilation fails with an error.

### Function Call

```
expression_list = expression ("," expression_list)?
function_call   = name:identifier "(" arguments:expression_list? ")"
```

If the named function is not a defined function and is not in the scope, 

### Unary Operation

```
unary_operator  = "!"|"-"|"~"
unary_operation = unary_operator expression
```

### Slice Operation

```
slice_operation = expression "[" start:expression ":" end:expression "]"
```

### Binary Operation

```
binary_operator  = "+" | "-" | "&&" | "||" | "==" | "!=" | ">" | "<" | ">=" | "<=" | "&" | "|" | "^" | "%" | "<<" | ">>"
binary_operator = binary_operator expression
```

## Statement

`statement = assertion | assignment | block`

### Assertion

`assertion = "verify" expression`

To execute an assertion, evaluate the expression. If the expression evaluates to a truthy value, execution of the program continues. If the expression evaluates to a falsy value, execution of the program fails.

### Satisfaction

`assertion = "satisfy" (instantiated_program | identifier)`

If an identifier is provided, and that identifier is not bound to an instantiated program in the current scope, compilation fails with an error.

TBD: explain

### Assignment

`assignment = "let" identifier "=" expression`

When executed, an assignment computes the value of the right-hand side expression, and binds the left-hand side identifier to that value. This assignment is valid until the end of the current [block](#block).

Reassignment is not permitted. If the identifier is already bound to another value in the current scope, compilation fails with an error.

### Blocks

`block = "{" statement* "}"`

To execute a block, execute each of its statements.

Ivy is _lexically scoped_: an [assignment](#assignment) is only valid until the end of the block in which the assignment occurs.

### Path

```
identifier_list = identifier ("," identifier_list)?
path            = "path" name:identifier? "(" parameters:identifier* ")" block`
```

Paths can be named or unnamed, i.e.:

```
path spend(a) {
  ...
}
```

```
path(a) {
  ...
}
```

Paths can be nested, and mixed with other types of statements:

```
path(a) {
  path(b) {
      ...
  }
  verify ...
  path(c) {
      ...
  }
}
```

When a block is executed, the caller chooses which of its paths should be executed. Only one of each block's top-level paths is executed.

The caller also specifies _path arguments_—values for each of the _path parameters_ defined by the path. Those identifiers are bound to those values for the scope of the path's block.

## Program

Programs must be _instantiated_ before they can be executed.

To execute an instantiated program, execute its block with each of the identifiers bound to the values that were provided at instantiation time.

### Program Definition

```
program_definition = "program" name:identifier "(" parameters:identifier_list? ")" block:block
```

A program definition binds the specified name to a block (and a list of parameters).


### Program Instantiation

```
instantiated_program = program_name:identifier "(" arguments:expression_list? ")"
```

To instantiate a program, evaluate each of the expressions in the arguments, and, in the scope of the instantiated program's block, bind those values to their corresponding parameters.

If the number of arguments does not match the number of parameters, compilation fails with an error.

To execute a program, execute its block with each of the identifiers bound to the values that were provided at instantiation time.

## Structs

### Struct Definition

```
type              = "string" | "int" | "list"
field             = field_type field_name
field_list        = field ("," field_list)?
struct_definition = "struct" struct_name:identifier "{" fields:field_list? "}"
```