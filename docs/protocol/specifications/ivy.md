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

### Identifier

```
identifier = [A-Za-z]+
```

#### Integer Literal

```
integer_literal = [0-9]+
```

#### Hex Literal

```
hex_digit   = [0-9] | [a-f]
hex_literal = "0x" (hex_digit hex_digit)*
```

## Values

TBD: coercion between strings, numbers, booleans

## Expressions

Expressions _evaluate_ to values.

TBD: Precedence

### Literal Expression

`literal_expression = integer_literal | hex_literal`

Literal expressions evaluate to the literal value they represent.

### Variable

```
variable = identifier
```

A variable evaluates to the value to which its identifier is bound in the current scope.

If the identifier is not bound to any value in the current scope, compilation fails with an error.

### Function Call

```
expression_list = expression ("," expression_list)?
function_call   = name:identifier "(" expression_list ")"
```

### Unary Operation

```
unary_operator  = "!"|"-"
unary_operation = unary_operator expression
```

### Binary Operation

```
binary_operator  = "+"|"-"|"&&"|"||"|"=="/"!="/">"/"<"/">="/"<="
binary_operator = unary_operator expression
```

## Statement

`statement = assertion | assignment | block`

### Assertion

`assertion = "verify" expression`

To execute an assertion, evaluate the expression. If the expression evaluates to a truthy value, execution of the program continues. If the expression evaluates to a falsy value, execution of the program fails.

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

```
program = "program" name:identifier "(" parameters:identifier_list ")" block:block
```

Programs must be _instantiated_ before they can be executed.

When a program is instantiated, the instantiator must provide _program arguments_—values to be assigned to each of its parameters. 

To executing a program, execute its block with each of the identifiers bound to the values that were provided at instantiation time.

