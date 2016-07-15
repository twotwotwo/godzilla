# godzilla

godzilla is a mutation testing tool for Go package. 

It is stil very much WIP but if you'd like to try it

    $ go get -u github.com/hydroflame/godzilla
    
then to run it:

    $ godzilla [PACKAGE]

## Mutators

### Swap If Else
The Swap If Else mutator swaps the body of an if/else statement

### Void Call Remover
The void call remover removes all the void function and method call.

### Conditionals Boundary
The conditionals boundary mutator swaps comparison operators to their counterpart that contains, or not, an equality sign.

| Original | New |
|----------|-----|
| >        | >=  |
| <        | <=  |
| >=       | >   |
| <=       | <   |

### Math
The math mutator swaps mathematical operators.

| Original | New |
|----------|-----|
| +	| - |
| -	| + |
| *	| / |
| /	| * |
| %	| * |
| &	| OR |
| OR | & |
| ^	| & |
| <<	| >> |
| >>	| << |
(markdown confuses `|` operator with table delimiters)

### Math Assign
The math assign mutator is similar to the Math mutator but for assignements.

| Original | New |
|----------|-----|
| += | -= |
| -= | += |
| *= | /= |
| /= | *= |
| %= | *= |
| &= | OR= |
| OR= | &= |
| ^= | &= |
| <<= | >>= |
| >>= | <<= |

(markdown confuses `|=` operator with table delimiters)

### Negate Conditionals
The negate conditionals mutator converts boolean checks to their inverse.

| Original | New |
|----------|-----|
| == | != |
| != | == |
| < | >= |
| <= | > |
| > | <= |
| >= | < |
