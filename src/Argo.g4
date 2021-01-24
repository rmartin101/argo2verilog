
grammar Argo;

// Rules

sourceFile
    : packageClause eos ( importClause eos )* ( functionDecl eos )* 
    ;

packageClause
    : 'package' IDENTIFIER
    ;

importClause
    : 'import' '(' STRING_LIT ')'
    ;

functionDecl
    : 'func' IDENTIFIER signature block
    ;

block
    : '{' statementList '}'
    ;

//StatementList
statementList
    : ( statement eos )*
    ;

// Regular statements 
statement
    : declaration
    | labeledStmt
    | simpleStmt
    | goStmt
    | deferStmt
    | returnStmt
    | breakStmt
    | continueStmt
    | gotoStmt
    | fallthroughStmt
    | block
    | ifStmt
    | switchStmt
    | selectStmt
    | forStmt
       ;

// Only simple statements can be used in if, swtich and for clauses 
simpleStmt
    : sendStmt
    | expressionStmt
    | incDecStmt
    | assignment
    | shortVarDecl
    | emptyStmt
    ;

declaration
    : varDecl
    ;

varDecl 
    : 'var' ( varSpec | '(' ( varSpec eos )* ')' )
    ;


shortVarDecl
    : identifierList ':=' expressionList
    ;

//varSpec
//    : identifierList ( r_type ( '=' expressionList )? | '=' expressionList )
//    ;

varSpec
    : identifierList r_type 
    ;


expressionList
    : expression ( ',' expression )*
    ;

identifierList
    : IDENTIFIER ( ',' IDENTIFIER )*
    ;

expressionStmt
    : expression
    ;

assignment
    : expressionList assign_op expressionList
    ;

ifStmt
    : 'if' (simpleStmt ';')? expression block ( 'else' ( ifStmt | block ) )?
    ;

switchStmt
    : exprSwitchStmt | typeSwitchStmt
    ;

//ExprSwitchStmt
exprSwitchStmt
    : 'switch' ( simpleStmt ';' )? expression? '{' exprCaseClause* '}'
    ;

exprCaseClause
    : exprSwitchCase ':' statementList
    ;

exprSwitchCase
    : 'case' expressionList | 'default'
    ;

typeSwitchStmt
    : 'switch' ( simpleStmt ';' )? typeSwitchGuard '{' typeCaseClause* '}'
    ;
typeSwitchGuard
    : ( IDENTIFIER ':=' )? primaryExpr '.' '(' 'type' ')'
    ;
typeCaseClause
    : typeSwitchCase ':' statementList
    ;
typeSwitchCase
    : 'case' typeList | 'default'
    ;
typeList
    : r_type ( ',' r_type )*
    ;


forStmt
    : 'for' ( expression | forClause | rangeClause )? block
    ;

forClause
    : simpleStmt? ';' expression? ';' simpleStmt?
    ;

rangeClause
    : (expressionList '=' | identifierList ':=' )? 'range' expression
    ;

goStmt
    : 'go' expression
    ;

deferStmt
    : 'defer' expression
    ;

sendStmt
    : expression '<-' expression
    ;

recvStmt
    : ( expressionList '=' | identifierList ':=' )? expression
    ;

selectStmt
    : 'select' '{' commClause* '}'
    ;
commClause
    : commCase ':' statementList
    ;
commCase
    : 'case' ( sendStmt | recvStmt ) | 'default'
    ;
    
//LabeledStmt = Label ":" Statement .
//Label       = identifier .
labeledStmt
    : IDENTIFIER ':' statement
    ;

//ReturnStmt = "return" [ ExpressionList ] .
returnStmt
    : 'return' expressionList?
    ;

//BreakStmt = "break" [ Label ] .
breakStmt
    : 'break' IDENTIFIER?
    ;

//ContinueStmt = "continue" [ Label ] .
continueStmt
    : 'continue' IDENTIFIER?
    ;

//GotoStmt = "goto" Label .
gotoStmt
    : 'goto' IDENTIFIER
    ;

//FallthroughStmt = "fallthrough" .
fallthroughStmt
    : 'fallthrough'
    ;

//IncDecStmt
incDecStmt
    : expression ( '++' | '--' )
    ;

//Type

assign_op
    : ('+' | '-' | '|' | '^' | '*' | '/' | '%' | '<<' | '>>' | '&' | '&^')? '='
    ;

emptyStmt
    : ';'
    ;

r_type
    : 'integer'
    | 'int'
    | 'uint'
    | 'float'
    | 'char'
    | 'short'
    | 'float'
    | 'double'
    | typeName
    | typeLit
    ;

typeName
    : IDENTIFIER
    ;

// need to add slices 
typeLit
    : arrayType
    | sliceType 
    | mapType
    | channelType
    | structType
    | pointerType 
    ;

arrayType
    : '[' arrayLength ']' elementType
    ;

sliceType
    : '[' ']' elementType
    ;

arrayLength
    : expression
    ;

mapType
    : 'map' '[' r_type ']' elementType
    ;

//ChannelType = ( "chan" | "chan" "<-" | "<-" "chan" ) ElementType .
channelType
    : ( 'chan' | 'chan' '<-' | '<-' 'chan' ) elementType
    ;

elementType
    : r_type
    ;

pointerType
    : '*' r_type
    ;


//    | expression BINARY_OP expression
expression
    : unaryExpr
    | expression ('||' | '&&' | '==' | '!=' | '<' | '<=' | '>' | '>=' | '+' | '-' | '|' | '^' | '*' | '/' | '%' | '<<' | '>>' | '&' | '&^') expression
    ;

unaryExpr
    : primaryExpr
    | ('+'|'-'|'!'|'^'|'*'|'&'|'<-') unaryExpr
    ;

primaryExpr
    : operand
    | conversion
    | primaryExpr selector
    | primaryExpr index
    | primaryExpr r_slice
    | primaryExpr typeAssertion
    | primaryExpr arguments
    ;

conversion
    : r_type '(' expression ','? ')'
    ;

selector
    : '.' IDENTIFIER
    ;

index
    : '[' expression ']'
    ;

r_slice
    : '[' (( expression? ':' expression? ) | ( expression? ':' expression ':' expression )) ']'
    ;

typeAssertion
    : '.' '(' r_type ')'
    ;

arguments
    : '(' ( ( expressionList | r_type ( ',' expressionList )? ) '...'? ','? )? ')'
    ;


//MethodExpr    = ReceiverType "." MethodName .
//ReceiverType  = TypeName | "(" "*" TypeName ")" | "(" ReceiverType ")" .
methodExpr
    : receiverType '.' IDENTIFIER
    ;

receiverType
    : typeName
    | '(' '*' typeName ')'
    | '(' receiverType ')'
    ;

//////////////////////////////////////////////////////////
operand
    : literal
    | operandName
    | methodExpr
    | '(' expression ')'
    ;

literal
    : basicLit
    | functionLit
    ;

basicLit
    : INT_LIT
    | FLOAT_LIT
    | STRING_LIT
    | HEX_LIT
    | OCTAL_LIT
    ;

operandName
    : IDENTIFIER
    ;

functionLit
    : 'func' function
    ;



function
    : signature block
    ;

signature 
    : parameters result
    | parameters 
    ;

result
    : parameters
    | r_type
    ;
    
parameters
    : '(' ( parameterList ','? )? ')'
    ;

parameterList
    : parameterDecl ( ',' parameterDecl )*
    ;

parameterDecl
    : identifierList? '...'? r_type
    ;


structType
    : 'struct' '{' ( fieldDecl eos )* '}'
    ;

fieldDecl
    : ( identifierList r_type) STRING_LIT?
    ;


eos
    : ';'
    | EOF
    ;


IDENTIFIER
    : LETTER ( LETTER | DECIMAL_DIGIT )*
    ;


LETTER
    : [a-zA-Z_]
    ;
    
//string_lit
fragment ESCAPED_QUOTE : '\\"';
STRING_LIT 
     :  '"' ( ESCAPED_QUOTE | ~('\n'|'\r') )*? '"'
     ;

// Integer literals

//int_lit     = decimal_lit | octal_lit | hex_lit .
INT_LIT
    : DECIMAL_LIT
    | OCTAL_LIT
    | HEX_LIT
    ;
//decimal_lit = ( "1" … "9" ) { DECIMAL_DIGIT } .
DECIMAL_LIT
    : [1-9] DECIMAL_DIGIT*
    ;

//octal_lit   = "0" { octal_digit } .
OCTAL_LIT
    : '0' OCTAL_DIGIT*
    ;

//hex_lit     = "0" ( "x" | "X" ) hex_digit { hex_digit } .
HEX_LIT
    : '0' ( 'x' | 'X' ) HEX_DIGIT+
    ;

// Floating-point literals

//float_lit = decimals "." [ decimals ] [ exponent ] |
//            decimals exponent |
//            "." decimals [ exponent ] .
FLOAT_LIT
    : DECIMALS '.' DECIMALS? EXPONENT?
    | DECIMALS EXPONENT
    | '.' DECIMALS EXPONENT?
    ;

//decimals  = decimal_digit { decimal_digit } .
DECIMALS
    : DECIMAL_DIGIT+
    ;

//exponent  = ( "e" | "E" ) [ "+" | "-" ] decimals .
EXPONENT
    : ( 'e' | 'E' ) ( '+' | '-' )? DECIMALS
    ;

// Imaginary literals
//imaginary_lit = (decimals | float_lit) "i" .
IMAGINARY_LIT
    : (DECIMALS | FLOAT_LIT) 'i'
    ;

//decimal_digit = "0" … "9" .
DECIMAL_DIGIT
    : [0-9]
    ;

//octal_digit   = "0" … "7" .
OCTAL_DIGIT
    : [0-7]
    ;

//hex_digit     = "0" … "9" | "A" … "F" | "a" … "f" .
HEX_DIGIT
    : [0-9a-fA-F]
    ;




//////////////////////////////////////////
// WHITESPACE: [ \r\n\t]+ -> skip
// Go uses whitespace as a separator so the lexer can't just skip everything 

WS  :  [ \t\n]+ -> channel(HIDDEN)
    ;


BlockComment
    :   '/*' .*? '*/'
        -> skip
    ;

LineComment
    :   '//' ~[\r\n]*
        -> skip
    ;
