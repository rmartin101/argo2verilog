/* Argo to Verilog Compiler 
    (c) 2019, Richard P. Martin and contributers 
    
    This program is free software: you can redistribute it and/or modify
    it under the terms of the GNU General Public License as published by
    the Free Software Foundation, either version 3 of the License, or
    (at your option) any later version.

    This program is distributed WITHOUT ANY WARRANTY; without even the implied warranty of
    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
    GNU General Public License Version 3 for more details.

    You should have received a copy of the GNU General Public License
    along with this program.  If not, see <https://www.gnu.org/licenses/>
*/


/* Convert a program in the Argo programming language to a Verilog executable */

/* Outline of the compiler 
  (1) Create and go-based abstract syntax tree (AST) using Antlr4 
  (2) use the AST to create a statement control flow graph (SCFG) 
  (3) Use the SCFG to create a Basic-Block CFG (BBCFG)
  (4) Optimize the BBCFG using data-flow analysis to increase parallelism 
  (5) Use the BBCFG to output the Verilog sections:
     
  A   Variable section --- creates all the variables 
  B   Channel section --- creates  all the channels. Each channel is a FIFO
  C   Map section --- create all the associative arrays. Each map is a verilog CAM
  D   Variable control --- 1 always block to control writes to each variable
  E   Control flow --- bit-vectors for control flow for each function. Each bit controls 1 basic block.  
*/

/* inferrng dual port BRAMS: https://danstrother.com/2010/09/11/inferring-rams-in-fpgas/
*/

package main

import (
	"fmt"
	"os"
	"flag"
	"strings"
	"strconv"
	"regexp"
	"bufio"
	"errors"
	"runtime"
	"sort"
	"log"
	// "bytes"
	"./parser"
	"github.com/antlr/antlr4/runtime/Go/antlr"
)

// housekeeping debugging functions
// use a simple assert with the line number to crash out with a stack trace 
// an assertion fails.

const NOTSPECIFIED = -1   // not specified, e.g. channel or map size 
const PARAMETER = -2      // variable is a parameter 

// force some control flow in some statements 
func pass() {

}

// an interval-based debugging level system
// 1 is general debug statements, higher is specific 
type DebugLog struct {
	flags map[string]bool 
}


func (d *DebugLog) init() {
	log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime)) // remove timestamp 
}
	
func (d *DebugLog) DbgLog(level string, format string, args ...interface{}) {
	var s1,s2 string 
	if (d.flags[level]) {
		s1 = fmt.Sprintf(format,args...)
		s2= fmt.Sprintf("%s: %s",level,s1)
		log.Print(s2)
	}
}


func assert(test bool, message string, location string, stackTrace bool) {
	fmt.Printf("Assertion failed at %s : cause: %s \n", location, message)
	if (stackTrace) {
		panic(message)
	}
}
// get the file name and line number of the file of this source code for
// error reporting 
func _file_line_() string {
    _, fileName, fileLine, ok := runtime.Caller(1)
    var s string
    if ok {
        s = fmt.Sprintf("%s:%d", fileName, fileLine)
    } else {
        s = ""
    }
    return s
}

// This is the representation of the AST we use 
type astNode struct {
	id int                // integer ID 
	ruleType string       // the type of the rule from the Argo.g4 definition
	isTerminal bool       // is a terminal node 
	parentID int          // parent integer ID
	childIDs []int        // list of child interger IDs 
	parent *astNode       // pointer to the parent 
	children []*astNode   // list of pointers to child nodes 
	sourceCode string     // the source code as a string
	sourceLineStart int     // start line in the source code
	sourceColStart  int     // start column in the source code
	sourceLineEnd   int     // ending line in the source code
	sourceColEnd   int     // ending column in the source code
	visited        bool    // flag for if this node is visited
}

// this is for the list of functions
type functionNode struct {
	id int               // ID of the function 
	funcName string     // name of the function
	fileName  string     // name of the source file 
	sourceRow  int        // row in the source code
	sourceCol  int        // column in the source code
	parameters  []*variableNode   // list of the variables that are parameters to this function
	parameterIDs []int            // list of variable node IDs 
	retVars    []*variableNode   // list of return variables
	retVarsIDs    []int         // list of return variables IDs 
	callers []*statementNode  // list of statements calling this function
	goCalls []*statementNode  // list of statements calling this function
}
	
// this is the object that holds a variable state 
type variableNode struct {
	id int                // every var gets a unique ID
	astDef     *astNode   // link to the astNode parent node
	astDefNum  int        // ID of the astNode parent definition
	astClass    string    // originating class
	isParameter bool      // is the a parameter to a function
	isResult bool      // is this a generated return value for the function
	goLangType  string    // numberic, channel, array or map 
	sourceName string     // name in the source code
	sourceRow  int        // row in the source code
	sourceCol  int        // column in the source code 
	funcName  string      // which function is this variable defined in
	primType string        // primitive type, e.g. int, float, uint.
	numBits     int           // number of bits in this variable
	canName string        // cannonical name for Verilog: package_row_col_func_name
	depth    int          // depth of a channel (number of element in the queue)               
	numDim   int          // number of dimension if an array
	dimensions []int      // the size of the dimensions 
	mapKeyType string     // type of the map key
	mapValType string     // type of the map value
	visited        bool    // flag for if this node is visited 
}


// holds the nodes for the statement control flow graph
// TODO: need sub-types for the different statement types 
type statementNode struct {
	id             int        // every statement gets an ID
	astDef         *astNode   // link to the astNode parent node of the statement type
	astDefID      int        // ID of the astNode parent definition
	astSubDef      *astNode   // the simplestatement type (e.g assignment, for, send, goto ...)
	astSubDefID    int        // ID of the simple statement type 
	stmtType     string     // the type of the simple statement 
	sourceName     string     // The source code of the statement 
	sourceRow      int        // row in the source code
	sourceCol      int        // column in the source code
	funcName       string      // which function is this statement is defined in
	readVars       []*variableNode  // variables read in this statement
	writeVars      []*variableNode  // variables written to in this statement 
	predecessors   []*statementNode // list of predicessors
	predIDs        []int       // IDs of the predicessors
	successors     []*statementNode // list of successors
	succIDs        []int       // IDs of the successors
	ifSimple *statementNode     // The enclosed block of sub-statements for the else clause
	ifTest   *statementNode     // The test expression 
	ifTaken  *statementNode     // The enclosed block of sub-statements for the taken part of an if
	ifElse   *statementNode     // The enclosed block of sub-statements for the else clause
	forInit *statementNode        // the for pre-statement 
	forCond   *statementNode     // the for test expression
	forPost   *statementNode      // the for post-statement
	forBlock  *statementNode     // the main block of the for statement 
	caseList   [][]*statementNode  // list of statements for a switch or select statement
	callTarget *statementNode     // regular caller target statement (funcDecl) 
	goTarget   *statementNode     // target of go statemetn (funcDecl)
	returnTarget []*statementNode  // list of return targets 
	visited        bool             // flag for if this node is visited
}

// Functions to add links in the statement graph
func (node *statementNode) addStmtSuccessor(succ *statementNode) {
	if (succ == nil ) {
		return 
	}
	node.successors = append(node.successors, succ)
	node.succIDs = append(node.succIDs, succ.id)	
}


func (node *statementNode) addStmtPredecessor(pred *statementNode) {
	if (pred == nil ) {
		return 
	}
	
	node.predecessors = append(node.predecessors, pred)
	node.predIDs = append(node.predIDs, pred.id)
	return 
}

// These statements return the statement IDs for various fields in the statement node
// so we dont have to store them in the node 
func (node *statementNode) ifSimpleID() int {
	if (node.ifSimple == nil) {
		return -1
	} else {
		s := node.ifSimple
		return s.id
	}
}

func (node *statementNode) ifTestID() int {
	if (node.ifTest == nil) {
		return -1
	} else {
		s := node.ifTest
		return s.id
	}
}

func (node *statementNode) ifTakenID() int {
	if (node.ifTaken == nil) {
		return -1
	} else {
		s := node.ifTaken
		return s.id
	}
}

func (node *statementNode) ifElseID() int {
	if (node.ifElse == nil) {
		return -1
	} else {
		s := node.ifElse
		return s.id
	}
}

// a control block represents a unit of control for execution, that is, a control bit 
// in the FPGA.
// After making a statement CFG, we make a control block CFG of smaller units. 
type controlBlockNode struct {
	id             int        // every control block gets an integer ID
	cntlDef        *statementNode // pointer back to this statement 
}


// these functions return event handlers counts for the parser.
// we care most about syntax errors, the others can occur in correct programs
// the program stops on syntax errors 
type ArgoErrorListener struct {
	syntaxErrors int
	ambiErrors int
	contextErrors int
	sensitivityErrors int 
}

func (l *ArgoErrorListener) SyntaxError(recognizer antlr.Recognizer, offendingSymbol interface{}, line, column int, msg string, e antlr.RecognitionException) {
	l.syntaxErrors += 1
}

func (l *ArgoErrorListener) ReportAmbiguity(recognizer antlr.Parser, dfa *antlr.DFA, startIndex, stopIndex int, exact bool, ambigAlts *antlr.BitSet, configs antlr.ATNConfigSet) {
	l.ambiErrors += 1
}

func (l *ArgoErrorListener) ReportAttemptingFullContext(recognizer antlr.Parser, dfa *antlr.DFA, startIndex, stopIndex int, conflictingAlts *antlr.BitSet, configs antlr.ATNConfigSet) {
	l.contextErrors += 1
}
func (l *ArgoErrorListener) ReportContextSensitivity(recognizer antlr.Parser, dfa *antlr.DFA, startIndex, stopIndex, prediction int, configs antlr.ATNConfigSet) {
	l.sensitivityErrors += 1
}



// this struct holds the state for the whole program

type argoListener struct {
	*parser.BaseArgoListener
	stack []int
	recog antlr.Parser
	logIt DebugLog //send items to the log 
	ProgramLines []string // the program as a list of strings, one string per line
	astNode2ID map[interface{}]int //  a map of the AST node pointers to small integer ID mapping
	nextAstID int                 // IDs for the AST nodes
	nextFuncID int                // IDs for the function nodes 
	nextVarID int                 // IDs for the Var nodes
	nextStatementID int           // IDs for the statement nodes
	nextBlockID int              // IDs for the basic Blocks 
	varNode2ID map[interface{}]int //  a map of the variable pointers to small integer ID mapping
	root *astNode                 // root of an absract syntax tree 
	astNodeList []*astNode        // list of nodes of an absract syntax tree, root has id = 0 
	varNodeList []*variableNode   // list of all variables in the program
	varNodeNameMap map[string]*variableNode  // map of cannonical names to variable nodes
	funcNodeList  []*functionNode     // list of functions
	funcNameMap map[string]*functionNode  //  maps the names of the functions to the function node 
	statementGraph   []*statementNode   // list of statement nodes. 
}

// get a node ID in the AST tree 
func (l *argoListener) getAstID(c antlr.Tree) int {

	// if the entry is in the table, return the integer ID
	if val, ok := l.astNode2ID[c] ; ok {
		return val 
	}

	// create a new entry and return it 
	l.astNode2ID[c] = l.nextAstID
	l.nextAstID = l.nextAstID + 1
	return 	l.astNode2ID[c]
}

// add a node to the list of all nodes
func (l *argoListener) addASTnode(n *astNode) {
	// need to check for duplicates
	l.astNodeList = append(l.astNodeList,n) 
}

func (l *argoListener) addVarNode(v *variableNode) {
	l.varNodeList = append(l.varNodeList,v) 
}

// Walk up the parents of the AST until we find a matching rule 
// assumes we have to back-up towards the root node
// Returns the first matching node 
func (node *astNode) walkUpToRule(ruleType string) *astNode {
	var foundit bool
	var parent *astNode
	
	foundit = false
	parent = node.parent
	// fmt.Printf("walkUpToRule Called rule: %s\n",ruleType)
	for (parent.id != 0 ) && (foundit == false) { 
		if (parent.ruleType == ruleType) {
			//fmt.Printf("walkUpToRule found match %s\n",ruleType)
			return parent 
		}
		parent = parent.parent 
	}

	fmt.Printf("Rule type %s for parents of node %d:%s not found \n", ruleType, node.id, node.ruleType,)
	return nil
}


// Walk down the AST until we find a matching rule. Use BFS order
// Returns the first matching node 
func (node *astNode) walkDownToRule(ruleType string) *astNode {
	var matched *astNode
	
	//fmt.Printf("walkDownToRule Called rule: %s node: ",ruleType)

	if (node == nil) {
		// fmt.Printf("nil \n")
		return nil
	}

	//fmt.Printf("%d\n", node.id)
	if (node.ruleType == ruleType) {
		//fmt.Printf("walkdowntorule returning %d \n", node.id)
		return node
	}
	
	for _, childNode := range node.children {
		matched = childNode.walkDownToRule(ruleType)
		if (matched != nil) {
			//fmt.Printf("walkdowntorule returning child %d\n", matched.id)
			return matched  // return the first match
		}
	}
	return nil
}

// get all the variables in an AST
// We go linearly through all the nodes looking for declaration types
// if we find one, we crawl the children to get the variable's name and type
// Each variable gets a canonical name which is the function name appended with the variable name  
// we can also crawl backward to get the scope

// Get a vardecl
// walk down to the ID list to get the names 
// walk down to r_type to get the type.
// add to the list of variables

// given and AST node of type r_type find the primitive type of the node
// also returns the number of bits of the type 
func (n *astNode) getPrimitiveType() (string,int) {
	var identifierType, identifierR_type *astNode
	var name,numB,nameB string  // number of bit as a string, name with no number 
	var numBits int   // number of bits for this type 

	// this should not happen
	if (n == nil) {
		fmt.Printf("Error at %s in getPrimitive type AST node line \n",_file_line_())
		return "",-1
	}

	numBits = 32 // default is 32 bits for variables 

	if (len(n.children) == 0){
		fmt.Printf("Error at %s no children in getPrimitive type node %d\n",_file_line_(),n.id)
		return "", -2
	}

	
	// for primitive types, this child is the type 
	identifierType = n.children[0]

	// if this is a typeLiteral, we must find the
	// the child r_type, then recurse down the tree
	// to find the primitive type 
	if (identifierType.ruleType == "typeLit")  {
		identifierR_type = identifierType.walkDownToRule("r_type")
		name, numBits = identifierR_type.getPrimitiveType()
		return name, numBits 
	}
	
	// if the child is a typename, go one level down
	if (identifierType.ruleType == "typeName")  {
		identifierType = identifierType.children[0]
	}

	// get the name and number of bits 
	name = identifierType.ruleType

	// pull out the numeric part
	reNum, _ := regexp.Compile("[0-9]+")
	// pull out the non-numeric part of the type name 
	reName, _ := regexp.Compile("[a-z]+")
	
	numB = reNum.FindString(name)
	nameB = reName.FindString(name)
	
	if (numB != "") {
		numBits, _ = strconv.Atoi(numB)
	}

	//fmt.Printf("get prim type returning %s %d\n",nameB,numBits)		
	return nameB,numBits
}

// return the dimension sizes of the array
// assumes we are at the arrayType Node in the AST graph
func (node *astNode) getArrayDimensions() ([] int) {
	var arrayLenNode, basicLitNode *astNode
	var dimensions []int
	var dimSize int
	
	dimensions = make([] int, 0)
	
	for _, child := range node.children {
		arrayLenNode = child.walkDownToRule("arrayLength")
		if arrayLenNode != nil {
			basicLitNode = arrayLenNode.walkDownToRule("basicLit")
			if (basicLitNode != nil) {
				dimSize, _  = strconv.Atoi(basicLitNode.children[0].ruleType)
				dimensions = append(dimensions,dimSize)
			} else {
				fmt.Printf("Error: at %s getting array dimensions AST node %d \n",_file_line_(),node.id)
			}
		}
	}
	if (dimensions == nil) {
		fmt.Printf("Error: at %s no array dimensions found AST node %d \n",_file_line_(),node.id)
	}
	return dimensions 
}

// get the map key and value types 
func (n *astNode) getMapKeyValus() (string,int,string,int) {
	
	return "",-1,"",-1
}


// get the number of elements in the channel
// or -1 if no size is found 
func (node *astNode) getChannelDepth() (int) {
	var queueSize int
	var basicLitNode *astNode

	queueSize = NOTSPECIFIED
	basicLitNode = node.walkDownToRule("basicLit")
	if (basicLitNode != nil) {
		queueSize, _  = strconv.Atoi(basicLitNode.children[0].ruleType)
	} else {
		//fmt.Printf("error: getting channel size AST node %d \n",node.id)
	}
	
	return queueSize
}

func (l *argoListener) getAllVariables() int {
	var funcDecl *astNode
	var identifierList,identifierR_type *astNode
	var funcName *astNode  // AST node of the function and function name
	var identChild *astNode // AST node for an identifier for the inferred type 
	// the three type of declarations are: varDecl (var keyword), parameterDecls (in a function signature), and shortVarDecls (:=)

	var varNameList []string
	var varNode     *variableNode 
	var varTypeStr string  // the type pf the var 
	var arrayTypeNode,channelTypeNode,mapTypeNode *astNode // if the variables are this class
	var numBits int        // number of bits in the type
	var depth int          // channel depth (size of the buffer) 
	var dimensions [] int  // slice which holds array dimensions 
	
	
	funcDecl = nil
	funcName = nil
	identifierList = nil
	varTypeStr = ""
	numBits = NOTSPECIFIED
	dimensions = nil
	depth = 1
	
	varNameList = make([] string, 1)     // list of names of the variables 
	arrayTypeNode = nil
	channelTypeNode = nil

	// for every AST node, see if it is a declaration
	// if so, name the variable the _function_name_name
	// for multiple instances of go functions, add the instance number
	astNodeLoop: 
	for _, node := range l.astNodeList {
		// find the enclosing function name
		if (node.ruleType == "varDecl") || (node.ruleType == "parameterDecl") || (node.ruleType == "shortVarDecl") {


			funcDecl = node.walkUpToRule("functionDecl")
			if (len(funcDecl.children) < 2) {  // need assertions here 
				fmt.Printf("Error at %s: no function name",_file_line_())
			}
			funcName = funcDecl.children[1]
			// now get the name and type of the actual declaration.
			// getting both the name and type depends on the kind of declaration it is 
			if ( (node.ruleType == "varDecl") || (node.ruleType== "parameterDecl") || (node.ruleType == "shortVarDecl"))  {

				// we dont know what the types are yet for this declaraion
				varNameList = nil
				arrayTypeNode = nil
				channelTypeNode = nil
				
				// find the list of identifiers as strings for these rules
				identifierList = node.walkDownToRule("identifierList")


				// if the identifierList is nil and the rule is a parameterdecl
				// these are the functions return parameters
				// We create special hidden vars for the return values in
				// the function parsing as the return variables 
				// are not named variables with AST nodes 
				if (identifierList == nil) {
					if (node.ruleType == "parameterDecl") {
						continue astNodeLoop ;
					}
					fmt.Printf("Error at %s: no identifier list",_file_line_())
					return 0
				}

				// get the type for this Decl rule
				identifierR_type = node.walkDownToRule("r_type")
				
				varTypeStr = ""; numBits = -1

				// if we assign a constant to a variable, we need to infer the
				// type of the constant which becomes the type of the variable 
				// TODO: need a better function to infer the type here
				if identifierR_type == nil {
					identifierR_type = node.walkDownToRule("basicLit")
					if identifierR_type != nil {
						identChild  =  identifierR_type.children[0]
						numStr := identChild.ruleType
						
						_, err := strconv.ParseInt(numStr,0,64)
						if err == nil {
							varTypeStr = "int"
							if ( (numStr[0] == byte("0"[0])) &&
								((numStr[1] == byte("x"[0])) || (numStr[1] == byte("X"[0])))) {
								numBits = 4*( len(numStr)-2) // make size = to number of digits 
							} else { 
								numBits = 32  // default size is 32 bit ints 
							}
						} else {
							_, err := strconv.ParseFloat(identChild.ruleType,32)
							if err == nil {
								varTypeStr = "float" 
							} else {
								fmt.Printf("primitive type failed for node %d\n",node.id )
							}
						}
 
					} else {  // if there is no name, this probably a return parameterDecl. 
                                                  // these dont have a name, so we need to make one up 
					}
					
				} else { 
					varTypeStr,numBits = identifierR_type.getPrimitiveType()
				}

				arrayTypeNode = node.walkDownToRule("arrayType")
				
				// check if these are arrays or channels 
				if ( arrayTypeNode != nil) {
					dimensions = arrayTypeNode.getArrayDimensions()
				} else {
					channelTypeNode = node.walkDownToRule("channelType")


					if ( channelTypeNode!= nil) {
						// channels in parameters do not have a depth
						// set to -2 as a flag for a channel in a
						// parameter 
						depth = -2
						if ((node.ruleType == "varDecl") || (node.ruleType == "shortVarDecl")) {
							// any literal as a child is used as the depth. This might not always work. 
							depth = node.getChannelDepth()
							// default to 1 if no depth is found 
							if (depth == NOTSPECIFIED) {
								depth = 1
							}
						}else {
							depth = PARAMETER
						}
					} else {
						mapTypeNode = node.walkDownToRule("mapType")
						if ( mapTypeNode!= nil) {
							// a map 
						}
					}
				}
				
				// create list of variable for all the children of this Decl rule 
				for _, child := range identifierList.children {
					if (child.ruleType != ","){
						varNameList = append(varNameList,child.ruleType)

					}

				}

				for _, varName := range varNameList {
					// fmt.Printf("found variable in func %s name: %s type: %s:%d",funcName.sourceCode,varName,varTypeStr,numBits)
					varNode = new(variableNode)
					varNode.id = l.nextVarID ; l.nextVarID++
					varNode.astDef = node
					varNode.astDefNum = node.id
					varNode.astClass = node.ruleType
					varNode.funcName = funcName.sourceCode
					varNode.sourceName  = varName
					varNode.sourceRow = node.sourceLineStart
					varNode.sourceCol = node.sourceColStart
					varNode.primType = varTypeStr
					varNode.numBits = numBits
					varNode.visited = false
					varNode.isParameter = false
					varNode.isResult = false 
					varNode.goLangType = "numeric"  // default 
					if (arrayTypeNode != nil) {
						varNode.dimensions = dimensions
						varNode.numDim = len(dimensions) 
						varNode.goLangType = "array"
						
					} 
					if (channelTypeNode != nil) {
						varNode.goLangType = "channel"
						varNode.depth = depth 
					}
					if (mapTypeNode != nil) {
						varNode.goLangType = "map"

					}
					
					if (node.ruleType== "parameterDecl") {
						varNode.isParameter = true 
					}
					
					// add this to a list of the variable nodes
					// for this program 
					l.addVarNode(varNode)
					
				}
				
				// Given the function name, type and variable names in the list
				// create a new variable node 
				
			} else if (node.ruleType == "shorVarDecl") {
				// short variable declaration 
			} else {
				fmt.Printf("Major Error\n ")
			}
		}

	}
	if (funcName == nil) {
		return 0
	}
	if (identifierList == nil) {
		return 0 
	}
	return 1
	
}


// parse an ifStmt AST node into a statement graph nodes
// return a list of lists of any sub-statements from the blocks 
// The structure is to create new statement nodes for all the childern in a main loop looking for
// the types of the children nodes. Then the function creates the predecessors and successor edges
func (l *argoListener) parseIfStmt(ifNode *astNode,funcDecl *astNode,ifStmt *statementNode) []*statementNode {
	var funcName *astNode               // name of the function
	var funcStr  string                 // name of the function as a string 
	var subNode        *astNode         // sub-simple statement type
	var ifSubStmtNode *astNode          // if we have an ifSubstatement, put a pointer to it here 
	
	var childStmt, simpleStmt, testStmt, takenStmt, elseStmt,subIfStmt *statementNode
	var seenBlockCount int;             // Counts blocks in the children. First It is the taken branch, second the else 
	var statements []*statementNode       // list of statmements 
	var slist []*statementNode          // list of statements for the block 
	childStmt = nil; simpleStmt = nil;  testStmt = nil; takenStmt = nil; elseStmt = nil; subIfStmt = nil 
	
	seenBlockCount = 0
	statements = nil

	// get the name of the function the ifStmt is in  
	funcName = funcDecl.children[1]
	funcStr = funcName.sourceCode

	// loop for each child and create the appropriate sub-statement node
	// after looping through all the children, we fix up the successors and predecessors edges
	ifNode.visited = true 
	for _, childNode := range ifNode.children {

		childStmt = nil
		// note the child statement can be nil around this loop if the type is not one of
		// simpleStmt, expression, block of ifStmt
		if (childNode.visited == false) {

			// create a new node and populate it, set the type later 
			if ( (childNode.ruleType == "simpleStmt") || (childNode.ruleType == "expression") ||
				(childNode.ruleType == "block") || (childNode.ruleType == "ifStmt") ) {
				childStmt = new(statementNode)
				childStmt.id = l.nextStatementID; l.nextStatementID++ 
				childStmt.astDef = childNode
				childStmt.astDefID =  childNode.id 
				childStmt.funcName = funcStr
				childStmt.sourceRow =  childNode.sourceLineStart 
				childStmt.sourceCol =  childNode.sourceColStart
				statements = append(statements,childStmt)
			}

			// set the node type 
			if (childNode.ruleType == "simpleStmt") {
				simpleStmt = childStmt

				subNode =  childNode.children[0]
				simpleStmt.stmtType = subNode.ruleType
				simpleStmt.astSubDef = subNode 
				simpleStmt.astSubDefID =  subNode.id

			}

			if (childNode.ruleType == "expression") {
				testStmt = childStmt
				subNode =  childNode.children[0]
				testStmt.stmtType = subNode.ruleType
				testStmt.astSubDef = subNode 
				testStmt.astSubDefID =  subNode.id
			}

			// if we have  block, recurse down the block to get the resulting statement list
			// also fix up the child to go to the statement, not the statementlist 
			if  (childNode.ruleType == "block") { 
				statementListNode := childNode.children[1] // get the list of statements in the block 
				slist = l.getListOfStatments(statementListNode,funcDecl)

				// get the list of statements from the statementlist 
				if (len(slist) >0)  {
					statements = append(statements,slist...)
					// connect the taken of else clause back to this node 
					head := slist[0]
					head.addStmtPredecessor(childStmt)
					childStmt.addStmtSuccessor(head)
				}
				


			}

			// this is the else block, unless the else is another if statement 
			if (seenBlockCount == 1)  && (childNode.ruleType == "block") {
				elseStmt = childStmt
				seenBlockCount = 2
				
				subNode =  childNode.children[1]
				elseStmt.stmtType = subNode.ruleType
				elseStmt.astSubDef = subNode 
				elseStmt.astSubDefID =  subNode.id

			}

			// this is the taken branch block
			// this has to come after the previous statement to make this work 
			if (seenBlockCount == 0) && (childNode.ruleType == "block") {
				takenStmt = childStmt
				seenBlockCount = 1 

				subNode =  childNode.children[1]
				takenStmt.stmtType = subNode.ruleType
				takenStmt.astSubDef = subNode 
				takenStmt.astSubDefID =  subNode.id

			}

			// this is the if () {block } else if {} construct when the else is an if statement 
			if (childNode.ruleType == "ifStmt") {

				ifSubStmtNode = childNode 
				subIfStmt = childStmt
				subIfStmt.stmtType = childNode.ruleType
				subIfStmt.astSubDef = nil 
				subIfStmt.astSubDefID = -1
				slist := l.parseIfStmt(ifSubStmtNode,funcDecl,subIfStmt)
				if (len(slist) >0) { 
					statements = append(statements,slist...)
					head := slist[0] 
					head.addStmtPredecessor(childStmt)
					subIfStmt.addStmtSuccessor(head)
				}
			
			}

		} // end if not visited 
		
	} // end for children of isStmt 

	// add links to the main if statement for the control flow graph 
	ifStmt.ifSimple = simpleStmt
	ifStmt.ifTest = testStmt
	ifStmt.ifTaken = takenStmt
	if (subIfStmt != nil) {
		ifStmt.ifElse = subIfStmt
	} else if (elseStmt != nil) {
		ifStmt.ifElse = elseStmt 
	}

	// Assertions that must hold for every if statements 
	if (testStmt == nil) {
		fmt.Printf("Error! at %s no test with if statement at AST node id %d\n", _file_line_(),ifNode.id)
		return nil 
	}

	if (takenStmt == nil) {
		fmt.Printf("Error! at %s no taken block with if statement at AST node id %d\n", _file_line_(),ifNode.id)
		return nil 
	}


	// santiy check, both the else an sub if statement can not be set 
	if (elseStmt != nil) && (subIfStmt != nil) {
		fmt.Printf("Error! at %s both else and if sub-statement set AST node id %d\n", _file_line_(),ifNode.id)
		fmt.Printf("Error! at %s statement len %d %s\n",_file_line_(),len(statements),statements)		
	}

	// This sets the pred and successor links for the simple statement 
	if (simpleStmt != nil) {
		simpleStmt.addStmtPredecessor(ifStmt)
		simpleStmt.addStmtSuccessor(testStmt)
		testStmt.addStmtPredecessor(simpleStmt)
	}

	// there must always be a test and taken statement 
	testStmt.addStmtSuccessor(takenStmt)
	takenStmt.addStmtPredecessor(testStmt)

	// the else and subIf are interchangable as the 2nd clause 
	if (elseStmt != nil) {
		testStmt.addStmtSuccessor(testStmt)
		elseStmt.addStmtPredecessor(testStmt)
	}

	if (subIfStmt != nil) {
		testStmt.addStmtSuccessor(subIfStmt)
		subIfStmt.addStmtPredecessor(testStmt)		
	}

	return statements 
}

// This parses a for statement.
// It tried to get the block and forClause first. Then it walks the children of the for clause and creates new
// statement nodes as it walks the forClause. The end of the function creates the edges between the statement nodes 
func (l *argoListener) parseForStmt(forNode *astNode,funcDecl *astNode,forStmt *statementNode) []*statementNode {
	var funcName *astNode               // name of the function
	var funcStr  string                 // name of the function
	var forClause  *astNode              //  if this statement has a for clause
	var forBlockNode   *astNode             // the block of statements for the for
	var subNode        *astNode         // sub-simple statement type
	var statements []*statementNode       // list of statmements 
	var slist []*statementNode          // list of statements for the block

	var childStmt *statementNode       
	var blockStmt, initStmt, conditionStmt, postStmt  *statementNode // statement nodes for the for statement 
	var seenSimple int
	
	statements = nil
	slist = nil
	
	// get the name of the function the ifStmt is in  
	funcName = funcDecl.children[1]
	funcStr = funcName.sourceCode // ToDO: change this to use the ruleType instead of the source code 

	// loop for each child and create the appropriate sub-statement node
	// after looping through all the children, we fix up the successors and predecessors edges
	forStmt.visited = true
	forClause = nil
	forBlockNode = nil
	blockStmt = nil
	seenSimple = 0  // how many simple statements have we seen?
	
	if (len(forNode.children) == 3 ) {
		forClause = forNode.children[1]
		forBlockNode =  forNode.children[2]

		statementListNode := forBlockNode.children[1] // get the list of statements in the block 
		slist = l.getListOfStatments(statementListNode,funcDecl)

		// get the list of statements from the statementlist 
		if (len(slist) >0)  {
			statements = append(statements,slist...)
			// connect the taken of else clause back to this node 
			head := slist[0]
			head.addStmtPredecessor(forStmt)
			blockStmt = head
			forStmt.addStmtSuccessor(head)
		} else {
			// should not happen, zero length block
			fmt.Printf("error, for statement with zero statement block \n")
		}
	} else {
		fmt.Printf("Error, at %s forstmt with wrong number of child at AST node %d \n",_file_line_(),forNode.id)
		return nil 
	}

	if (forClause == nil) {
		fmt.Printf("Error! at %s empty forClause in AST ID %d \n",_file_line_(),forNode.id)
		return nil 
	}
	
	for _, childNode := range forClause.children {

		childStmt = nil
		// note the child statement can be nil around this loop if the type is not one of
		// simpleStmt, expression, block of ifStmt
		if (childNode.visited == false) {

			// create a new node and populate it, set the type later 
			if ( (childNode.ruleType == "simpleStmt") || (childNode.ruleType == "expression") ) { 
				childStmt = new(statementNode)
				
				childStmt.id = l.nextStatementID; l.nextStatementID++ // create new ID
				childStmt.astDef = childNode
				childStmt.astDefID =  childNode.id 
				childStmt.funcName = funcStr
				childStmt.sourceRow =  childNode.sourceLineStart 
				childStmt.sourceCol =  childNode.sourceColStart
				statements = append(statements,childStmt)
			}

			if (childNode.ruleType == "expression") {
				conditionStmt = childStmt
				subNode =  childNode.children[0]
				conditionStmt.stmtType = subNode.ruleType
				conditionStmt.astSubDef = subNode 
				conditionStmt.astSubDefID =  subNode.id
			}

			// the second simple statement is the post-condition 
			if (seenSimple == 1) && (childNode.ruleType == "simpleStmt")  {
				postStmt = childStmt

				subNode =  childNode.children[0]
				postStmt.stmtType = subNode.ruleType
				postStmt.astSubDef = subNode 
				postStmt.astSubDefID =  subNode.id

				seenSimple++

			}

			// the first simple statement is the initialization statement 
			if (seenSimple == 0 ) && (childNode.ruleType == "simpleStmt")  {
				initStmt = childStmt

				subNode =  childNode.children[0]
				initStmt.stmtType = subNode.ruleType
				initStmt.astSubDef = subNode 
				initStmt.astSubDefID =  subNode.id

				seenSimple++ 

			}

		} // end if not visited 
		
	} // end for children of forClause 

	// add links to the main if statement for the control flow graph 
	forStmt.forInit = initStmt
	forStmt.forCond = conditionStmt
	forStmt.forPost = postStmt 

	// Assertions that must hold for every if statements 
	if (conditionStmt == nil) {
		fmt.Printf("Error! at %s condition for for statement at AST node id %d\n", _file_line_(),forNode.id)
		return nil 
	}

	// This sets the pred and successor links for the initialization statement
	// note this assume the block is defined, which is might not be
	
	if (initStmt != nil) {
		initStmt.addStmtPredecessor(forStmt)
		initStmt.addStmtSuccessor(conditionStmt)
		conditionStmt.addStmtPredecessor(initStmt)
	}

	if (conditionStmt != nil) {
		// there must always be a block statement 
		conditionStmt.addStmtSuccessor(blockStmt)
		blockStmt.addStmtPredecessor(conditionStmt)
	} else {
		blockStmt.addStmtPredecessor(forStmt)
	}

	if (postStmt != nil) {
		postStmt.addStmtPredecessor(blockStmt)
		blockStmt.addStmtSuccessor(postStmt)
	}
	
	return statements 
}

func (l *argoListener) parseSwitchStmt(switchnode *astNode,funcDecl *astNode) [][]*statementNode {
	return nil
}


func (l *argoListener) parseSelectStmt(selectnode *astNode,funcDecl *astNode) [][]*statementNode {
	return nil
}


// create a return variable node. When a return happens, we store the value
// in this generated variable and treat the return as an expression with
// additional control flow 
func (l *argoListener) makeReturnVariable(identifierR_type *astNode,funcName string) *variableNode {
	var varTypeStr string
	var numBits int 
	var retVarNode *variableNode

	retVarNode = nil
	varTypeStr,numBits = identifierR_type.getPrimitiveType()
	
	if (varTypeStr != "") {
		retVarNode = new (variableNode)
		retVarNode.id = l.nextVarID ; l.nextVarID++		
		
		lineStartStr := strconv.Itoa(identifierR_type.sourceLineStart)
		colStartStr := strconv.Itoa(identifierR_type.sourceColStart)
		
		retVarNode.astDef = identifierR_type 
		retVarNode.astDefNum = identifierR_type.id 
		retVarNode.astClass =  identifierR_type.ruleType
		retVarNode.funcName = funcName
		retVarNode.sourceName  = "_" + funcName + "_" + lineStartStr + "_" + colStartStr  + "_"
		retVarNode.sourceRow = identifierR_type.sourceLineStart
		retVarNode.sourceCol = identifierR_type.sourceColStart
		retVarNode.primType = varTypeStr
		retVarNode.numBits = numBits
		retVarNode.visited = false
		retVarNode.isParameter = false
		retVarNode.isResult = true 
		retVarNode.goLangType = "numeric"  // default

		l.varNodeList = append(l.varNodeList,retVarNode)
		
		
	} else {
		fmt.Printf("Error: at %s no type information for return variable\n",_file_line_())		
	}
	return retVarNode
}

// get a list of all the functions
// assumes variables are already parsed to look up the parameters 
func (l *argoListener) getAllFunctions() {
	var funcName *astNode    // name of the function -- AST node 
	var funcStr string       // name of the function as a string. Must be unique
	var resultNode *astNode  // node for the result 
	var retParams *astNode    // the parameters for the return values for a function call
	var identifierR_type *astNode  // node for getting the primitive type 
	var fNode *functionNode   // node of the function we are creating 

	var retVarNode *variableNode // the variable node for the return value 
	
	// get parameters assumes we have the variables already parsed
	if (len(l.varNodeList) <= 0) {
		fmt.Printf("Error: at %s, warning no variables in getallfunctions\n",_file_line_())
	}

	for i, funcDecl := range l.astNodeList {
		if (funcDecl.ruleType == "functionDecl") {
			if (len(funcDecl.children) < 2) {  // need assertions here 
				fmt.Printf("Error at %s: %d no function name",_file_line_(),i)
			}
			funcName = funcDecl.children[1]
			funcStr = funcName.ruleType
			if (len(funcStr) > 0) {
				fNode = new(functionNode)
				fNode.id = l.nextFuncID; l.nextFuncID++
				fNode.funcName = funcStr
				fNode.sourceRow = funcName.sourceLineStart
				fNode.sourceCol = funcName.sourceColStart
				l.funcNameMap[funcStr] = fNode

				// get the parameters 
				for _, varNode := range (l.varNodeList) {
					if ((varNode.funcName == fNode.funcName) && (varNode.isParameter) ) {
						fNode.parameters = append(fNode.parameters,varNode)
						fNode.parameterIDs = append(fNode.parameterIDs,varNode.id)
					}
				}
				// get the return values and add them to the list of variables
				resultNode = funcDecl.walkDownToRule("result")
				if (resultNode != nil) { 
					retParams = resultNode.walkDownToRule("parameterList")

					if (retParams == nil) { // this is case we have single parameter
						identifierR_type = resultNode.walkDownToRule("r_type")
						retVarNode =  l.makeReturnVariable(identifierR_type,funcStr)

					} else { // we have a parameter list 
						for _, typeNode := range retParams.children {
							identifierR_type = typeNode.walkDownToRule("r_type")
							if (identifierR_type == nil) { continue }  // skip 
							retVarNode =  l.makeReturnVariable(identifierR_type,funcStr)
							if (retVarNode == nil) {
								fmt.Printf("Error making return var node\n")
							}
						}
					}
					
				} else {
					// function does not have any results 
				}
				
			} else {	
				fmt.Printf("Error at %s: AST node %d zero length function name\n",_file_line_(),i)
			}
			
		}
	}
}

// get both the forward and backward edges for function calls in the statement graph 
func (l *argoListener) getCallsandReturns() {

	}


// get the list of go routines and add edges in the statement graph 
func (l *argoListener) getGoRoutines() {

}

// Given a statementlist, return a list of statementNodes
// Uses recursion to follow if and for, case and select statements
func (l *argoListener) getListOfStatments(listnode *astNode,funcDecl *astNode) []*statementNode {
	var funcName *astNode  // name of the function for the current statement
	var funcStr  string   //  string name of the function
	var subNode *astNode  // current statement node
	var stmtTypeNode *astNode // which simpleStmt type is this?, e.g ifStmt, shortVarDecl, forStmt.
	var statementList []*statementNode
	var statements []*statementNode 
	var predecessorStmt *statementNode 
	var stateNode *statementNode
	
	//var numChildren int
	
	if (len(funcDecl.children) < 2) {  // need assertions here 
		fmt.Printf("Major Error")
	}
	funcName = funcDecl.children[1]
	funcStr = funcName.sourceCode
	

	// top level traversal of the statement list

	predecessorStmt = nil
	//numChildren = len(listnode.children) 
	for _, childnode := range listnode.children { // for each statement in the statementlist

		// go one level down to skip the variable declaration statements
		if (len(childnode.children) > 0) {
			
			subNode = childnode.children[0] // subnode should be a statement

			// skip decls 
			if (subNode.ruleType != "declaration" )&& (subNode.ruleType != ";") && (len(subNode.children) >0) {

				// simple statements have to go one level down to get the actual type 
				if (subNode.ruleType == "simpleStmt") { 
					stmtTypeNode = subNode.children[0]
				} else { 
					stmtTypeNode = subNode
				}
				
				// create a new statement node if we have not visited the originating AST statement node
				if (childnode.visited == false ){
					stateNode = new(statementNode)

					stateNode.id = l.nextStatementID; l.nextStatementID++
					stateNode.astDef = childnode
					stateNode.astDefID =  childnode.id
					stateNode.astSubDef = stmtTypeNode
					stateNode.astSubDefID =  stmtTypeNode.id
					stateNode.stmtType = stmtTypeNode.ruleType
					stateNode.funcName = funcStr
					stateNode.sourceRow =  stmtTypeNode.sourceLineStart 
					stateNode.sourceCol =  stmtTypeNode.sourceColStart
					stateNode.ifSimple = nil 
					stateNode.ifTaken = nil
					stateNode.ifElse  = nil
					stateNode.forInit = nil
					stateNode.forCond = nil 
					stateNode.forPost = nil
					stateNode.forBlock = nil
					stateNode.caseList = nil
					
					childnode.visited = true

					// attach the predecessor to the newly generated node
					if (predecessorStmt != nil) {
						predecessorStmt.addStmtSuccessor(stateNode)
						stateNode.addStmtPredecessor(predecessorStmt)
					}

					// Get sub statement lists for this node
					switch stateNode.stmtType { 
					case "declaration": 
					case "labeledStmt":
						
					case "goStmt":
					case "returnStmt":
					case "breakStmt":
					case "continueStmt":
					case "gotoStmt":
					case "fallthroughStmt":
					case "ifStmt":
						statements = l.parseIfStmt(subNode,funcDecl,stateNode)
						if (len(statements) >0) {
							statementList = append(statementList,statements...)
						}
						
					case "switchStmt":
					case "selectStmt":
					case "forStmt":
						statements = l.parseForStmt(subNode,funcDecl,stateNode)
						if (len(statements) >0) {
							statementList = append(statementList,statements...)
						}
					case "sendStmt":
					case "expressionStmt":
					case "incDecStmt":
					case "assignment":
					case "shortVarDecl":
					case "emptyStmt":

					default:
						fmt.Printf("Major error: no such statement type\n")
					}

					statementList = append(statementList, stateNode)
					predecessorStmt = statementList[len(statementList)-1] ;
					
				} // end if visited == false 
			} // end if child is not a declaration or statement separator 
			
		} // end if number of chidren >0
		
		// add links to previous/next for the sub-nodes 
	} // end for the children nodes

	if (statementList == nil) {
		fmt.Printf("\t \tError list is nil \n")
		return nil 
	}
	
	return statementList 
}



// Generate a control flow graph (CFG) at the statement level.
// We look for statement lists. If we find one, back up to the
// enclosing function to find the function def to use as and entry
// point. Then recursively decend down the statement list
// Assumes there is only one statement list per function (fixme)

func (l *argoListener) getStatementGraph() int {
	var funcDecl *astNode  // Function Declaration 
	var funcName *astNode  // name of the function for the current statement
	var funcStr  string   //  string name of the function
	var entryNode *statementNode // function declation is the entry node in the graph 
	var statements []*statementNode // list of statement nodes
	var numLists int

	// mark all nodes as not visited 
	for _, node := range l.astNodeList {
		node.visited = false
	}

	numLists = 0
	// now make a pass over the graph 
	for _, astnode := range l.astNodeList {

		// if we find a statement list, start building the statement graph 
		if (astnode.ruleType == "statementList") {
			// add the function declaration to the statement graph as the
			// entry point for the statements in the function
			// this entry point becomes the copy-in for parameters in the block-graph 
			funcDecl = astnode.walkUpToRule("functionDecl")
			if (len(funcDecl.children) < 2) {  // need assertions here 
				fmt.Printf("Major Error")
			}
			funcName = funcDecl.children[1]
			funcStr = funcName.ruleType // RPM-was sourcecode 
			// We need to include function declarations in the statement graph because
			// they are the place node of copying in the arguments in the graph.
			if (funcDecl.visited == false ) {
				entryNode = new(statementNode)
				entryNode.id = l.nextStatementID; l.nextStatementID++
				entryNode.astDef = funcDecl
				entryNode.astDefID =  funcDecl.id
				entryNode.astSubDef = nil 
				entryNode.astSubDefID =  0
				entryNode.stmtType = funcDecl.ruleType
				entryNode.funcName = funcStr
				entryNode.sourceRow =  funcDecl.sourceLineStart 
				entryNode.sourceCol =  funcDecl.sourceColStart
				funcDecl.visited = true
				statements = make([]*statementNode,1)
				statements[0] = entryNode 
				statements = append(statements,l.getListOfStatments(astnode,funcDecl)...)
				// add the statements to the global graph 
				l.statementGraph = append(l.statementGraph,statements...)
				// set the edges from the entry point to this list

				// fix the predecessor/successor edges 
				if ( len(statements) > 1) {
					entryNode.addStmtSuccessor(statements[1])
					nextHead := statements[1]
					nextHead.addStmtPredecessor(entryNode)
				}


			} else {
				//fmt.Printf("Graph: AST Node %d was visited \n",astnode.id)
				pass()
			}
			
		}
		
	}
	
	return numLists
}

// print all the AST nodes. Can be in rawWithText mode, which includes the source code with each node, or
// in dotShort mode, which is a graphViz format suitable for making graphs with the dot program 
func (l *argoListener) printASTnodes(outputStyle string) {

	var nodeStr string // name of the AST node 
	
	if (outputStyle == "rawWithText") { 
		for _, node := range l.astNodeList {
			fmt.Printf("AST Nodes: %d: %s ::%s:: @(%d,%d),(%d,%d) parent: %d children: ", node.id, node.ruleType, node.sourceCode, node.sourceLineStart, node.sourceColStart, node.sourceLineEnd, node.sourceColEnd, node.parentID )
 			for _, childID := range node.childIDs {
				fmt.Printf("%d ",childID)
			}
			fmt.Printf("\n")
		}
	}

	// output the AST graph in Dot-Format
	// first build an ID to name map, then print the map in dot format

	if (outputStyle == "dotShort") {
		nodeID2Name := make(map[int] string)

		// build a map of IDs to names 
		for _, node := range l.astNodeList {
			nodeStr = strconv.Itoa(node.id) + ":" + node.ruleType
			if len(node.sourceCode) <= 5 {
				nodeStr = nodeStr+":"+node.sourceCode
			}
			// remove quotes
			nodeStr = strings.Replace(nodeStr,"\"","",-1)
			// qoute everthing else 
			nodeID2Name[node.id] = "\"" + nodeStr + "\""  
		}
		// now print the graph 
		fmt.Printf("Digraph G { \n") 
		for _, node := range l.astNodeList {
			if len(node.childIDs) > 0 { 
				nodeStr = nodeID2Name[node.id]
				for _, childID := range node.childIDs {
					fmt.Printf("\t %s -> %s ; \n", nodeStr,nodeID2Name[childID] )
					
				}
			}
		}
		fmt.Printf("}\n") 
	}
}

func (l *argoListener) printVarNodes() {

	for _, node := range l.varNodeList {
		fmt.Printf("Variable:%d name:%s func:%s pos:(%d,%d) class:%s prim:%s size:%d param:%t result:%t ",
			node.id,node.sourceName,node.funcName,node.sourceRow,node.sourceCol,
			node.goLangType,node.primType, node.numBits,node.isParameter,node.isResult)
		switch (node.goLangType) {

		case "array":
			fmt.Printf("dimensions: ")
			for i,size := range node.dimensions {
				fmt.Printf(" %d:%d ",i+1,size)
			}
		case "map":
		case "channel":
			fmt.Printf("depth %d ",node.depth)
		case "numeric":
		}
		fmt.Printf("\n")
	}
}

// print the statement graph
func (l *argoListener) printStatementGraph() {
	var j int

	// sort by id number 
	sort.Slice(l.statementGraph, func(i, j int) bool {
		return l.statementGraph[i].id < l.statementGraph[j].id
	})
	
	for i, node := range l.statementGraph {
		fmt.Printf("Stmt: %d: ID:%d at (%d,%d) type:%s pred: ", i,node.id, node.sourceRow, node.sourceCol, node.stmtType)
		// assertion checks:
		if (len(node.predecessors) != len(node.predIDs)) {
			fmt.Printf("Error: length of precedessors does not match %d %d \n",len(node.predecessors),len(node.predIDs))
		}
		if (len(node.successors) != len(node.succIDs)) {
			fmt.Printf("Error: length of successors does not match %d %d \n",len(node.successors),len(node.succIDs))
		}

		for _,id := range node.predIDs { 
			fmt.Printf("%d ",id)
		}

		fmt.Printf(" succ: ")
		j = 0 
		for (j < len(node.succIDs)) {
			fmt.Printf("%d ",node.succIDs[j])
			j++
		}		

		// Get sub statement lists for this node
		switch node.stmtType { 
		case "declaration": 
		case "labeledStmt":
		case "goStmt":
		case "returnStmt":
		case "breakStmt":
		case "continueStmt":
		case "gotoStmt":
		case "fallthroughStmt":
		case "ifStmt":
			fmt.Printf("simple: %d test: %d taken %d else %d ",node.ifSimpleID(),node.ifTestID(),node.ifTakenID(),node.ifElseID())
		case "switchStmt":
		case "selectStmt":
		case "forStmt":
		case "sendStmt":
		case "expressionStmt":
		case "incDecStmt":
		case "assignment":
		case "shortVarDecl":
		case "emptyStmt":

		default:
			pass()
		}

		fmt.Printf("\n")		
	}
}

// recursive function to visit nodes in the Antlr4 graph 
func VisitNode(l *argoListener,c antlr.Tree, parent *astNode,level int) astNode {
	var progText string
	var err error
	var id int 
	var isTerminalNode bool
	var startline, startcol,stopline, stopcol int 

	startline =0; startcol =0; stopline=0; stopcol =0;
	
	mylevel := level + 1
	id = l.getAstID(c)
	progText = ""
	isTerminalNode = false
	_ ,ok1 := c.(antlr.TerminalNode)
	if ok1 {
		progText = antlr.TreesGetNodeText(c,nil,l.recog)
		isTerminalNode = true 
	}
	
	t3,ok2 := c.(antlr.ParserRuleContext) 
	if ok2 {
		start := t3.GetStart()
		stop := t3.GetStop()
		startline = start.GetLine()
		startcol = start.GetColumn()
		stopline = stop.GetLine()
		stopcol = stop.GetColumn()
		progText,err = rowscols2String(l.ProgramLines,startline,startcol,stopline,stopcol)
		if (err != nil) {
			//fmt.Printf("RowCols error on program text %s %d:%d to %d:%d ",err,startline,startcol,stopline,stopcol)
			progText = "ERR"
		}
	}
	
	ruleName := antlr.TreesGetNodeText(c,nil,l.recog)

	thisNode := astNode{id : id , ruleType : ruleName, parentID: parent.id, parent: parent, sourceCode: progText , isTerminal : isTerminalNode, sourceLineStart: startline, sourceColStart : startcol, sourceLineEnd : stopline, sourceColEnd : stopcol }
	thisNode.visited = false
	
	for i := 0; i < c.GetChildCount(); i++ {
		child := c.GetChild(i)
		childASTnode := VisitNode(l,child,&thisNode,mylevel)
		thisNode.children = append(thisNode.children,&childASTnode) 
		thisNode.childIDs = append(thisNode.childIDs,childASTnode.id)
	}
	
	l.addASTnode(&thisNode)
	return thisNode
}


// EnterStart creates the AST by crawling the whole tree
// it leaves a list of AST nodes in the listener struct sorted by ID
func (l *argoListener) EnterSourceFile(c *parser.SourceFileContext) {
	var level int
	var id int
	
	level = 0
	id = l.getAstID(c)

	// get the root AST node
	root := astNode{ id : id, parentID : 0, parent : nil , ruleType: "SourceFile", sourceCode : "WholeProgramText" } 


	
	for i := 0; i < c.GetChildCount(); i++ {
		child := c.GetChild(i)
		// fmt.Printf(" child %d: %p \n",i,child)
		childNode := VisitNode(l,child,&root,level)
		root.children = append(root.children, &childNode)
		root.childIDs = append(root.childIDs, childNode.id)		
		
	}
	// add the root back in
	l.root = &root
	l.addASTnode(&root)

	// sort all the nodes by nodeID in the list of nodes 
	sort.Slice(l.astNodeList, func(i, j int) bool {
		return l.astNodeList[i].id < l.astNodeList[j].id
	})

	
}

// rowscols2String takes a program as a list of lines, a start row,col and
// end row,col pair an returns the interving text as a single string 
func rowscols2String (lineArray []string, startline,startcol,endline,endcol int) (string, error) {
	var row, begincol,lastcol int
	var currLine, retString string

	// fmt.Printf("rowscols got: %d %d %d %d \n",startline,startcol,endline,endcol)
	// Sanity check, must have at least 1 line
	if len(lineArray) < 1 {
		return retString, errors.New("line array too short")
	}
	retString = ""

	// Sanity check, lines are numbered from 1 to N 	
	if (startline < 1) {
		return retString, errors.New("startline <1 ")
	}

	// for each row, grab the text
	// Note that rows in the Go array start a zero
	// but in the compiler text numbers start at 1
	// columns start a zero
	for row = startline-1; row <= endline-1; row ++ {

		if row >= len(lineArray) {
			return retString, errors.New("bad row too long")						
		}
		
		currLine = lineArray[row]
		begincol = 0
		lastcol = len(currLine)
		// might have to truncate the first column 
		if (row == (startline-1)) {
			begincol = startcol
		}
		// might have to truncate the last column
		// add 1 to make Go's exclusive end work 
		if (row == (endline-1)) {
			lastcol = endcol+1
		}

		// santity check 
		if (begincol < 0) || (lastcol > len(currLine)) {
			return retString, errors.New("bad column")
		}

		
		// append the line segment to the current string
		//fmt.Printf("row: %d column start end %d %d \n", row, begincol, lastcol)

		// substrings are begining inclusive but end exclusive, so add 1
		// to the last column

		// 2nd santity check
		// sometime we have an AST node with no program text
		if (begincol > lastcol ) {
			return retString, errors.New("begining is greater than end! ")
		}
		
		retString =  retString + currLine[begincol:lastcol]
	}

	// this should not happen, but if does its an error 
	if (retString == "") {
		return retString, errors.New("no program string found\n")

	}
	
	return retString,nil
}

// getFileLines takes a file name and returns an array of strings.
// Each string is one line of text from the file.
// This allows converting from row, column format to a single string
// of the source code. 
func getFileLines(fname string) ([]string, error) {
	var retLines [] string
	var line string
	
	file, err := os.Open(fname)
	if err != nil {
		fmt.Printf("getFileLines Error opening file %s\n",fname)
		return nil,err 
	}

	// remember to close the file 
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line = scanner.Text()
		retLines = append(retLines,line)
		// fmt.Printf("Got Line:%s\n",line)
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("getFileLines non-fatal Error");
		return nil,err
	}
	
	return retLines,nil
}

func (l *argoListener) EnterPackageClause(c *parser.PackageClauseContext) {
	//fmt.Printf("entering Package\n")
}

func (l *argoListener) EnterImportClause(c *parser.ImportClauseContext) {
	//fmt.Printf("entering import\n")
}
	

// parseArgo takes a string expression and returns the root node of the resulting AST
func parseArgo(fname *string) *argoListener {

	var err error
	var listener *argoListener

	listener = new(argoListener)
	
	input, err := antlr.NewFileStream(*fname)
	
	lexer := parser.NewArgoLexer(input)
	errorCount := new(ArgoErrorListener)
	lexer.AddErrorListener(errorCount)
	
	stream := antlr.NewCommonTokenStream(lexer,0)

	p := parser.NewArgoParser(stream)
	p.AddErrorListener(errorCount)
	
	listener.recog = p
	progLines, err2 := getFileLines(*fname)
	if (err2 != nil) {
		fmt.Printf("Whoaa! didn't program lines")
		
	}
	listener.ProgramLines = progLines

	listener.nextAstID = 0
	listener.astNode2ID = make(map[interface{}]int)

	listener.nextVarID = 0
	listener.varNode2ID = make(map[interface{}]int)

	listener.nextStatementID = 0
	listener.nextBlockID = 0

	listener.funcNameMap = make(map[string]*functionNode)
	
	listener.logIt.flags = make(map[string]bool,16)
	listener.logIt.init()
	listener.logIt.flags["MIN"] = true
	
	if (err != nil) {
		fmt.Printf("Getting program lines failed\n")
		os.Exit(-1)
	}

	listener.logIt.DbgLog("MIN","testing the log %d %d %d \n",5,10,20)
	
	// Finally parse the expression (by walking the tree)
	antlr.ParseTreeWalkerDefault.Walk(listener, p.SourceFile())
	
	if (errorCount.syntaxErrors > 0) {
		fmt.Printf("Parsing of program halted due to syntax errors \n");
		os.Exit(1)

	}
	return listener
}

func main() {
	var parsedProgram *argoListener 
	var inputFileName_p *string
	var printASTasGraphViz_p,printVarNames_p,printStmtGraph_p *bool
	
	printASTasGraphViz_p = flag.Bool("gv",false,"print AST in GraphViz format")
	printVarNames_p = flag.Bool("vars",false,"print all variables")
	printStmtGraph_p = flag.Bool("stmt",false,"print statement graph")
	inputFileName_p = flag.String("i","input.go","input file name")

	flag.Parse()


	parsedProgram = parseArgo(inputFileName_p)
	parsedProgram.getAllVariables()  // must call get all variables first 
	parsedProgram.getAllFunctions()  // then get all functions 
	parsedProgram.getStatementGraph()  // now make the statementgraph 
	
	if (*printASTasGraphViz_p) {
		parsedProgram.printASTnodes("rawWithText")
		//parsedProgram.printASTnodes("dotShort")
	}

	if (*printVarNames_p) {
		parsedProgram.printVarNodes()
		
	}

	if (*printStmtGraph_p) {
		parsedProgram.printStatementGraph()	
	}


	
}
