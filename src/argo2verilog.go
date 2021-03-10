/* Argo to Verilog Compiler 
    (c) 2020, Richard P. Martin and contributers 
    
    This program is free software: you can redistribute it and/or modify
    it under the terms of the GNU General Public License as published by
    the Free Software Foundation, either version 3 of the License, or
    (at your option) any later version.

    This program is distributed WITHOUT ANY WARRANTY; without even the implied warranty of
    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
    GNU General Public License Version 3 for more details.t

    You should have received a copy of the GNU General Public License
    along with this program.  If not, see <https://www.gnu.org/licenses/>
*/


/* Convert a program in the Argo programming language to a Verilog executable */

/* Outline of the compiler 
  (1) Create and go-based parse tree using Antlr4 
  (2) use the parse tree to create a statement control flow graph (SCFG) 
  (3) Use the SCFG to create a Basic-Block CFG (BBCFG)
  (4) Optimize the BBCFG using data-flow analysis to increase parallelism 
  (5) Use the BBCFG to output the Verilog sections:
     
  A   Variable section --- creates all the variables 
  B   Channel section --- creates  all the channels. Each channel is a FIFO
  C   Map section --- create all the associative arrays. Each map is a verilog CAM
  D   Variable control --- 1 always block to control writes to each variable
  E   Control flow --- bit-vectors for control flow for each function. Each bit controls 1 basic block.  
*/

/* see: https://ruslanspivak.com/lsbasi-part7/ */

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
func Pass() {

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
    _, fileName, fileLine, ok := runtime.Caller(1)  // get the file,line of previous stack frame 
    var s string
    if ok {
        s = fmt.Sprintf("%s:%d", fileName, fileLine)
    } else {
        s = ""
    }
    return s
}

/* ***************  graph nodes structures definition section   ********************** */

// This is the representation of the parse tree nodes
type ParseNode struct {
	id int                // integer ID 
	ruleType string       // the type of the rule from the Argo.g4 definition
	isTerminal bool       // is a terminal node 
	parentID int          // parent integer ID
	childIDs []int        // list of child interger IDs 
	parent *ParseNode       // pointer to the parent 
	children []*ParseNode   // list of pointers to child nodes 
	sourceCode string     // the source code as a string
	sourceLineStart int     // start line in the source code
	sourceColStart  int     // start column in the source code
	sourceLineEnd   int     // ending line in the source code
	sourceColEnd   int     // ending column in the source code
	visited        bool    // flag for if this node is visited
}

// this is for the list of functions
type FunctionNode struct {
	id int               // ID of the function 
	funcName string     // name of the function
	fileName  string     // name of the source file 
	sourceRow  int        // row in the source code
	sourceCol  int        // column in the source code
	parameters  []*VariableNode   // list of the variables that are parameters to this function
	parameterIDs []int            // list of variable node IDs 
	retVars    []*VariableNode   // list of return variables
	retVarsIDs    []int         // list of return variables IDs 
	callers []*StatementNode  // list of statements calling this function
	goCalls []*StatementNode  // list of statements calling this function
}
	
// this is the object that holds a variable state 
type VariableNode struct {
	id int                // every var gets a unique ID
	parseDef     *ParseNode   // link to the ParseNode parent node
	parseDefNum  int        // ID of the ParseNode parent definition
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
	canName string        // cannonical name for Verilog: name_func_row_col
	depth    int          // depth of a channel (number of element in the queue)               
	numDim   int          // number of dimension if an array
	dimensions []int      // the size of the dimensions 
	mapKeyType string     // type of the map key
	mapValType string     // type of the map value
	cfgNodes  []*CfgNode  // control flow nodes for data-flow 
	visited        bool    // flag for if this node is visited 
}

// a scope is a set of local variable names to global name mappings
type VarScope struct {
	id int ;   // id of this scope
	varNameMap map[string]*VariableNode  // map of source code names to  cannonical names
	statements []*StatementNode         // list of statements in this scope 
	cfgNodes  []*CfgNode  // control flow nodes which are in this scope 	
	visited        bool    // flag if visited 	
}


// holds the nodes for the statement control flow graph
// The statement graph is modeled on a control flow graph. However, we model blocks as
// linear set of statements with parents and children. That is, a statement block is a 
// linear sequence of predecessors and successors. Sub blocks are children. A sub-block is
// defined by a type, such as a for, if or case.
// Every block has a parent statement which is the usual entry into the blocks, except for a goto 
// break can continue statementes escape to the parent block
// TODO: need sub-types for the different statement types

type StatementNode struct {
	id             int        // every statement gets an ID
	parseDef         *ParseNode   // link to the ParseNode parent node of the statement type
	parseDefID      int        // ID of the ParseNode parent definition
	parseSubDef      *ParseNode   // the simplestatement type (e.g assignment, for, send, goto ...)
	parseSubDefID    int        // ID of the simple statement type 
	stmtType     string     // the type of the simple statement 
	sourceName     string     // The source code of the statement 
	sourceRow      int        // row in the source code
	sourceCol      int        // column in the source code
	funcName       string      // which function is this statement is defined in
	vScope  *VarScope         // list of variables in the scope of this statement 
	readVars       []*VariableNode  // variables read in this statement
	writeVars      []*VariableNode  // variables written to in this statement
	predecessors   []*StatementNode // list of predicessors
	predIDs        []int       // IDs of the predicessors
	successors     []*StatementNode // list of successors
	succIDs[]int       // IDs of the successors
	parent         *StatementNode // the parent node for this block
	parentID       int            // the parent node ID for this block
	child         *StatementNode // If there is a child node for this block. Defines scope 
	childID       int            // Child ID for this block 
	ifSimple *StatementNode     // The enclosed block of sub-statements for the else clause
	ifTest   *StatementNode     // The test expression 
	ifTaken  *StatementNode     // The enclosed block of sub-statements for the taken xpart of an if
	ifElse   *StatementNode     // The enclosed block of sub-statements for the else clause
	ifRoot   *StatementNode    // root if stmt if this a simple, test, taken or else node 
	forInit *StatementNode        // the for pre-statement 
	forCond   *StatementNode     // the for test expression
	forPost   *StatementNode      // the for post-statement
	forBlock  *StatementNode     // the main block of the for statement
	forTail    *StatementNode     //  end of the for block
	forRoot   *StatementNode       // root for stmt if this is an init, cond post or block 
	caseList   [][]*StatementNode  // list of statements for a switch or select statement
	callTargets []*StatementNode     // regular caller target statement (funcDecl)
	callers []*StatementNode         // which statements call into this node
	goTargets   []*StatementNode     // target of go statemetn (funcDecl)
	returnTargets []*StatementNode  // list of return targets
	cfgNodes    []*CfgNode          // list of control flow graph nodes for this statement 
	visited        bool             // flag for if this node is visited
}

// hold the control flow graph. Each control flow node is a verilog always block
// The control bits are edges into or out of the node. The bits are checked as conditions
// needed to enter or bits are set on exit

type CfgNode struct {
        id int                           // the integer ID of this node
	cfgType string                   // the type of this node. assignment, test, call, return
	cannName string                  // cannonical name for this node: package,function,sourceRow,sourceCol,subNum
	statement *StatementNode         // the go statement for this control node
	stmtID    int                    // ID number of the statement node
	subStmt   *StatementNode         // the sub-statement for If, For and Case statements
	subStmtID  int                   // the sub-statementID
	isDummy    bool                  // dummy node to use as a placeholder to build the cfg graph 
	sourceRow      int               // row in the source code
	sourceCol      int               // column in the source code
	successors []*CfgNode             // successive statement
        successors_taken []*CfgNode      // for For and if statements - following statement if the condition is false
	test *StatementNode              // the test for an if stateent 
	predecessors []*CfgNode          // nodes that could come before this one
        predecessors_taken []*CfgNode     // taken ifs that could come before this one
        call_target   *CfgNode           // for a return, the possible gosub sources 
        returnTargets []*CfgNode         // for a return, the possible nodes to return to 
        readVars [] *VariableNode        // variables read by the node 
        writeVars [] *VariableNode       // vartiable written by the node 
	verilog   []* string              // the verilog to output 
        visited bool                     // for graph traversal, if visited or not
}

// Functions to add links in the statement graph
func (node *StatementNode) addStmtSuccessor(succ *StatementNode) {
	if (succ == nil ) {
		return 
	}
	node.successors = append(node.successors, succ)
	node.succIDs = append(node.succIDs, succ.id)	
}


func (node *StatementNode) addStmtPredecessor(pred *StatementNode) {
	if (pred == nil ) {
		return 
	}
	
	node.predecessors = append(node.predecessors, pred)
	node.predIDs = append(node.predIDs, pred.id)
	return 
}

func (node *StatementNode) setStmtPredNil() {
	node.predecessors = nil
	node.predIDs = nil
}

func (node *StatementNode) setStmtSuccNil() {
	node.successors = nil
	node.succIDs = nil
}

// These statements return the statement IDs for various fields in the statement node
// so we dont have to store them in the node
// If statement branchs to IDs 
func (node *StatementNode) ifSimpleID() int {
	if (node.ifSimple == nil) {
		return -1
	} else {
		s := node.ifSimple
		return s.id
	}
}

func (node *StatementNode) ifTestID() int {
	if (node.ifTest == nil) {
		return -1
	} else {
		s := node.ifTest
		return s.id
	}
}

func (node *StatementNode) ifTakenID() int {
	if (node.ifTaken == nil) {
		return -1
	} else {
		s := node.ifTaken
		return s.id
	}
}

func (node *StatementNode) ifElseID() int {
	if (node.ifElse == nil) {
		return -1
	} else {
		s := node.ifElse
		return s.id
	}
}

// For statement branchs to IDs 
func (node *StatementNode) forInitID() int {
	if (node.forInit == nil) {
		return -1
	} else {
		s := node.forInit
		return s.id
	}
}

func (node *StatementNode) forCondID() int {
	if (node.forCond == nil) {
		return -1
	} else {
		s := node.forCond
		return s.id
	}
}

func (node *StatementNode) forPostID() int {
	if (node.forPost == nil) {
		return -1
	} else {
		s := node.forPost
		return s.id
	}
}

func (node *StatementNode) forBlockID() int {
	if (node.forBlock == nil) {
		return -1
	} else {
		s := node.forBlock
		return s.id
	}
}

func (node *StatementNode) forTailID() int {
	if (node.forTail == nil) {
		return -1
	} else {
		s := node.forTail
		return s.id
	}
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

	if (l.syntaxErrors > max_parse_errors) {
		fmt.Printf("Error: too many syntax errors at %s . Aborting. \n",_file_line_())
		os.Exit(-1)
	}
}

func (l *ArgoErrorListener) ReportAmbiguity(recognizer antlr.Parser, dfa *antlr.DFA, startIndex, stopIndex int, exact bool, ambigAlts *antlr.BitSet, configs antlr.ATNConfigSet) {
	l.ambiErrors += 1

	if (l.ambiErrors > max_parse_errors) {
		fmt.Printf("Error: too many ambigutity errors at %s . Aborting. \n",_file_line_())
		os.Exit(-1)
	}

}

func (l *ArgoErrorListener) ReportAttemptingFullContext(recognizer antlr.Parser, dfa *antlr.DFA, startIndex, stopIndex int, conflictingAlts *antlr.BitSet, configs antlr.ATNConfigSet) {
	l.contextErrors += 1

	if (l.contextErrors > max_parse_errors) {
		fmt.Printf("Error: too many context errors at %s . Aborting. \n",_file_line_())
		os.Exit(-1)
	}

}
func (l *ArgoErrorListener) ReportContextSensitivity(recognizer antlr.Parser, dfa *antlr.DFA, startIndex, stopIndex, prediction int, configs antlr.ATNConfigSet) {
	l.sensitivityErrors += 1

	if (l.sensitivityErrors > max_parse_errors) {
		fmt.Printf("Error: too many sensitvity rrors at %s . Aborting. \n",_file_line_())
		os.Exit(-1)		
	}
}


// this is the main top-level structure 
// it holds the state for the whole program
type argoListener struct {
	*parser.BaseArgoListener
	stack []int
	recog antlr.Parser
	logIt DebugLog //send items to the log 
	ProgramLines []string // the program as a list of strings, one string per line
	ParseNode2ID map[interface{}]int //  a map of the AST node pointers to small integer ID mapping
	nextParseID int                  // IDs for the AST nodes
	nextFuncID int                   // IDs for the function nodes 
	nextVarID int                    // IDs for the Var nodes
	nextStatementID int              // IDs for the statement nodes
	nextCfgID int                    // IDs for the control flow nodes 
	root *ParseNode                  // root of an absract syntax tree 
	ParseNodeList []*ParseNode        // list of nodes of an absract syntax tree, root has id = 0 
	varNodeList []*VariableNode       // The symbol table -- a list of all variables in the program. 
	funcNodeList  []*FunctionNode     // list of functions
	funcNameMap map[string]*FunctionNode  //  maps the names of the functions to the function node 
	statementGraph   []*StatementNode   // list of statement nodes.
	controlFlowGraph []*CfgNode         // list of control flow nodes
	debugFlags     uint64               // flags for debugging. 1 = verilog control 
	moduleName    string                // name of the module for Verilog/VHDL
	outputFile       *os.File            // output file writer
	
}

// get a node ID in the AST tree 
func (l *argoListener) getAstID(c antlr.Tree) int {

	// if the entry is in the table, return the integer ID
	if val, ok := l.ParseNode2ID[c] ; ok {
		return val 
	}

	// create a new entry and return it 
	l.ParseNode2ID[c] = l.nextParseID
	l.nextParseID = l.nextParseID + 1
	return 	l.ParseNode2ID[c]
}

// add a node to the list of all nodes
func (l *argoListener) addParseNode(n *ParseNode) {
	// need to check for duplicates
	l.ParseNodeList = append(l.ParseNodeList,n) 
}

func (l *argoListener) addVarNode(v *VariableNode) {
	l.varNodeList = append(l.varNodeList,v) 
}

// Walk up the parents of the AST until we find a matching rule 
// assumes we have to back-up towards the root node
// Returns the first matching node 
func (node *ParseNode) walkUpToRule(ruleType string) *ParseNode {
	var foundit bool
	var parent *ParseNode
	
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
func (node *ParseNode) walkDownToRule(ruleType string) *ParseNode {
	var matched *ParseNode
	
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

// Walk down the AST until we find all matching rules. Return a list of all such rules 
func (node *ParseNode) walkDownToAllRules(ruleType string) []*ParseNode {
	//fmt.Printf("walkDownToallRules Called rule: id %d %s node: \n ",node.id,ruleType)
	var childList []*ParseNode
	
	if (node == nil) {
		return nil
	}

	ruleList := make([]*ParseNode,0)
	if (node.ruleType == ruleType) {
		ruleList = append(ruleList,node)
		return ruleList 
	}
	
	for _, childNode := range node.children {
		if (childNode != nil) {
			childList = childNode.walkDownToAllRules(ruleType)
			if (ruleList != nil) {
				ruleList = append(ruleList,childList...)
				// fmt.Printf("walkdowntoallrules len is %d \n",len(ruleList))
			}
			
		}
	}
	if (len(ruleList) == 0) {
		return nil 
	} else { 
		return ruleList
	}
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
func (n *ParseNode) getPrimitiveType() (string,int) {
	var identifierType, identifierR_type *ParseNode
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
func (node *ParseNode) getArrayDimensions() ([] int) {
	var arrayLenNode, basicLitNode *ParseNode
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
func (n *ParseNode) getMapKeyValus() (string,int,string,int) {
	
	return "",-1,"",-1
}


// get the number of elements in the channel
// or -1 if no size is found 
func (node *ParseNode) getChannelDepth() (int) {
	var queueSize int
	var basicLitNode *ParseNode

	queueSize = NOTSPECIFIED
	basicLitNode = node.walkDownToRule("basicLit")
	if (basicLitNode != nil) {
		queueSize, _  = strconv.Atoi(basicLitNode.children[0].ruleType)
	} else {
		//fmt.Printf("error: getting channel size AST node %d \n",node.id)
	}
	
	return queueSize
}

// return a variable node by the package, function and variable name 
func (l *argoListener) getVarNodeByNames(packageName,funcName,varName string) *VariableNode {

	// TODO: add packages to the name-spaces 
	if (packageName != "") {
		fmt.Printf("Warning: Package namespaces not supported yet\n")
	}

	// TODO: need a hash map ist
	for _, varNode := range l.varNodeList {
		
		if ((varNode.funcName == funcName) && (varNode.sourceName == varName)) {
			return varNode 
		}
	}
	
	return nil
}

// get a function node by string name 
func (l *argoListener) getFuncNodeByNames(packageName,funcName string) *FunctionNode {

	// TODO: add packages to the name-spaces 
	if (packageName != "") {
		fmt.Printf("Warning: Package namespaces not supported yet\n")
	}

	
	// TODO: need a hash map ist
	for _, funcNode := range l.funcNodeList {
		
		if (funcNode.funcName == funcName) {
			return funcNode
		}
	}
	
	return nil
}

// given a parse node, compute the variables for that node 
func (l *argoListener) getParseVariables( node *ParseNode) []*VariableNode {

	var returnVarList []*VariableNode // list of vars to return 
	var funcDecl *ParseNode
	var identifierList,identifierR_type *ParseNode
	var funcName *ParseNode  // AST node of the function and function name
	var identChild *ParseNode // AST node for an identifier for the inferred type 
	// the three type of declarations are: varDecl (var keyword), parameterDecls (in a function signature), and shortVarDecls (:=)

	var varNameList []string
	var varNode     *VariableNode 
	var varTypeStr string  // the type pf the var 
	var arrayTypeNode,channelTypeNode,mapTypeNode *ParseNode // if the variables are this class
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

	returnVarList = make([]*VariableNode,0)
	
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
					//continue ParseNodeLoop ;
					return returnVarList 
				}
				fmt.Printf("Error at %s: no identifier list",_file_line_())
				return returnVarList
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
						if (len(numStr) >= 2) {
							if ( (numStr[0] == byte("0"[0])) &&
								((numStr[1] == byte("x"[0])) || (numStr[1] == byte("X"[0])))) {
								numBits = 4*( len(numStr)-2) // make size = to number of digits 
							} else { 
								numBits = 32  // default size is 32 bit ints 
							}
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
				varNode = new(VariableNode)
				varNode.id = l.nextVarID ; l.nextVarID++
				varNode.parseDef = node
				varNode.parseDefNum = node.id
				varNode.astClass = node.ruleType
				varNode.funcName = funcName.sourceCode
				varNode.sourceName  = varName
				varNode.sourceRow = node.sourceLineStart
				varNode.sourceCol = node.sourceColStart
				varNode.canName = varName + "_" + funcName.sourceCode + "_" + strconv.Itoa(node.sourceLineStart) + "_" + strconv.Itoa(node.sourceColStart)
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
				//l.addVarNode(varNode)
				returnVarList = append(returnVarList,varNode)
				
			}
				
			// Given the function name, type and variable names in the list
			// create a new variable node 
			
		} else if (node.ruleType == "shorVarDecl") {
			// short variable declaration 
		} else {
				fmt.Printf("Major Error\n ")
		}
	} else {
		// wrong node type 
	}

	return returnVarList 
}


func (l *argoListener) getAllVariables() int {
	var funcDecl *ParseNode
	var identifierList,identifierR_type *ParseNode
	var funcName *ParseNode  // AST node of the function and function name
	var identChild *ParseNode // AST node for an identifier for the inferred type 
	// the three type of declarations are: varDecl (var keyword), parameterDecls (in a function signature), and shortVarDecls (:=)

	var varNameList []string
	var varNode     *VariableNode 
	var varTypeStr string  // the type pf the var 
	var arrayTypeNode,channelTypeNode,mapTypeNode *ParseNode // if the variables are this class
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
	ParseNodeLoop: 
	for _, node := range l.ParseNodeList {
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
						continue ParseNodeLoop ;
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
							if (len(numStr) >= 2) {
								if ( (numStr[0] == byte("0"[0])) &&
									((numStr[1] == byte("x"[0])) || (numStr[1] == byte("X"[0])))) {
									numBits = 4*( len(numStr)-2) // make size = to number of digits 
								} else { 
									numBits = 32  // default size is 32 bit ints 
								}
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
					varNode = new(VariableNode)
					varNode.id = l.nextVarID ; l.nextVarID++
					varNode.parseDef = node
					varNode.parseDefNum = node.id
					varNode.astClass = node.ruleType
					varNode.funcName = funcName.sourceCode
					varNode.sourceName  = varName
					varNode.sourceRow = node.sourceLineStart
					varNode.sourceCol = node.sourceColStart
					varNode.canName = varName + "_" + funcName.sourceCode + "_" + strconv.Itoa(node.sourceLineStart) + "_" + strconv.Itoa(node.sourceColStart)
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


// given an expression, return the same expression with the variables replaced by
// one with their canonical global names 

func (l *argoListener) flattenVarsInExpression() string {

	return "1+1"
}

// Rules for edge dangles:
// Returns always jump to the exit node
// If statements ends jump to the sucessor of the If statement
// for statements return to the top of the for statement 
// continue return to the outer for statement
// break jumps to the successor of the enclosing for statement

// link the 
func (l *argoListener) linkDangles(parentHead,parentTail *StatementNode) int {
	var nextChild *StatementNode
	var childTail *StatementNode 
	// var forPost *StatementNode
	// get the successor of the parent.

	var count int

	
	if (parentHead == nil) {
		fmt.Printf("Error! linkDangles called with nil parent %s\n", _file_line_())		
		return 0 
	}

	
	if (parentHead.child == nil) {
		return 0 
	}

	if (parentHead.visited == true) {
		return 0 
	}

	count = 0
	nextChild = parentHead.child
	childTail = nil
	parentHead.visited = true

	// find the tail of the 
	for {
		switch nextChild.stmtType { 
		case "declaration": 
		case "labeledStmt":
		case "goStmt":
		case "returnStmt":
		case "breakStmt":
		case "continueStmt":
		case "gotoStmt":
		case "fallthroughStmt":
		case "ifStmt": // go down each branch to set the links to the successor if this if statement 
			if (nextChild.ifTaken != nil) {
				count += l.linkDangles(nextChild.ifTaken,nextChild.successors[0])
			}
			if (nextChild.ifElse != nil) {
				count += l.linkDangles(nextChild.ifElse,nextChild.successors[0])
			}						
		case "switchStmt":
		case "selectStmt":
		case "forStmt":
			if (nextChild.child != nil) {
				count += l.linkDangles(nextChild,nextChild.successors[0])
			}									
		case "sendStmt":
		case "expressionStmt":
		case "incDecStmt":
		case "assignment":
		case "shortVarDecl":
		case "emptyStmt":

		default:
			Pass()
		}
		

		if (len(nextChild.successors) > 0) {  
			nextChild = nextChild.successors[0] // walk to the next successor 
		} else { // no succesor, link to the target
			childTail = nextChild
			if (parentHead.stmtType == "forStmt") {
				childTail.addStmtSuccessor(parentHead) // loop back to top of the for loop
			} else { 
				parentTail.addStmtPredecessor(childTail)  // fall through to the outer statement 
				childTail.addStmtSuccessor(parentTail)   
			}
			count ++
			break 
		}
	} // end for 

	return count 
}

// parse an ifStmt AST node into a statement graph nodes
// return a list of lists of any sub-statements from the blocks 
// The structure is to create new statement nodes for all the childern in a main loop looking for
// the types of the children nodes. Then the function creates the predecessors and successor edges
func (l *argoListener) parseIfStmt(ifNode *ParseNode,funcDecl *ParseNode,ifStmt *StatementNode,eosStmt *StatementNode) []*StatementNode {
	var funcName *ParseNode               // name of the function
	var funcStr  string                 // name of the function as a string 
	var subNode        *ParseNode         // sub-simple statement type
	var ifSubStmtNode *ParseNode          // if we have an ifSubstatement, put a pointer to it here 
	
	var childStmt, simpleStmt, testStmt, takenStmt, elseStmt,subIfStmt *StatementNode
	var takenHead,takenTail, elseHead,elseTail, headStmt, tailStmt *StatementNode
	
	var seenBlockCount int;             // Counts blocks in the children. First It is the taken branch, second the else 
	var statements []*StatementNode       // list of statmements 
	var blocklist []*StatementNode          // list of statements for the block 
	childStmt = nil; simpleStmt = nil;  testStmt = nil; takenStmt = nil; elseStmt = nil; subIfStmt = nil 
	
	seenBlockCount = 0
	statements = nil

	// get the name of the function the ifStmt is in  
	funcName = funcDecl.children[1]
	funcStr = funcName.sourceCode
	ifSubStmtNode = nil 
	// loop for each child and create the appropriate sub-statement node
	// after looping through all the children, we fix up the successors and predecessors edges
	ifNode.visited = true 
	for _, childNode := range ifNode.children {

		childStmt = nil
		// note the child statement can be nil around this loop if the type is not one of

		// simpleStmt, expression, block of ifStmt
		if (childNode.visited == false) {

			childNode.visited = true 

			// create a new node and populate it, set the type later 
			if ( (childNode.ruleType == "simpleStmt") || (childNode.ruleType == "expression") ||
				(childNode.ruleType == "ifStmt")) {
				childStmt = new(StatementNode)
				childStmt.id = l.nextStatementID; l.nextStatementID++ 
				childStmt.parseDef = childNode
				childStmt.parseDefID =  childNode.id 
				childStmt.funcName = funcStr
				childStmt.sourceRow =  childNode.sourceLineStart 
				childStmt.sourceCol =  childNode.sourceColStart
				childStmt.parent = ifStmt
				childStmt.parentID = ifStmt.id
				
				// set the variable scope to be the same as the if statement's scope 
				childStmt.vScope = ifStmt.vScope 
				childStmt.vScope.statements = append(childStmt.vScope.statements,childStmt)
				
				// add this to the global list of statements 
				l.statementGraph = append(l.statementGraph,childStmt)

			}

			// set the node type 
			if (childNode.ruleType == "simpleStmt") {
				simpleStmt = childStmt

				subNode =  childNode.children[0]
				simpleStmt.stmtType = subNode.ruleType
				simpleStmt.parseSubDef = subNode 
				simpleStmt.parseSubDefID =  subNode.id

			}

			if (childNode.ruleType == "expression") {
				testStmt = childStmt
				subNode =  childNode.children[0]
				testStmt.stmtType = subNode.ruleType
				testStmt.parseSubDef = subNode 
				testStmt.parseSubDefID =  subNode.id
			}

			// if we have  block, recurse down the block to get the resulting statement list
			// also fix up the child to go to the statement, not the statementlist 
			if  (childNode.ruleType == "block") { 
				statementListNode := childNode.children[1] // get the list of statements in the block 
				blocklist = l.getListOfStatements(statementListNode,ifStmt,funcDecl)
				
				// get the list of statements from the statementlist 
				if (len(blocklist) >0)  {
					// statements = append(statements,slist...) dont make global list 
					// connect the taken of else clause back to this node 
					headStmt = blocklist[0]
					tailStmt = blocklist[len(blocklist)-1] 
				}
			}

			// this is the else block, unless the else is another if statement 
			if (seenBlockCount == 1)  && (childNode.ruleType == "block") {
				elseStmt = headStmt
				seenBlockCount = 2
				elseHead = headStmt
				elseTail = tailStmt 
			}

			// this is the taken branch block
			// this has to come after the previous statement to make this work 
			if (seenBlockCount == 0) && (childNode.ruleType == "block") {
				takenStmt = headStmt
				seenBlockCount = 1 
				takenHead = headStmt
				takenTail = tailStmt
			}

			// this is the if () {block } else if {} construct when the else is an if statement 
			if (childNode.ruleType == "ifStmt") {
				childStmt.stmtType = "ifStmt"
				ifSubStmtNode = childNode
				subIfStmt = childStmt
				blocklist = l.parseIfStmt(ifSubStmtNode,funcDecl,subIfStmt,eosStmt)
				if (len(blocklist) >0) {
					elseHead = blocklist[0]
					elseTail = blocklist[len(blocklist)-1]
				}
			}
		} // end if not visited 
		
	} // end for children of isStmt 

	
	// add links to the main if statement for the control flow graph 
	ifStmt.ifSimple = simpleStmt
	ifStmt.ifTest = testStmt
	ifStmt.ifTaken = takenStmt

	// Assertions that must hold for every if statements 
	if (testStmt == nil) {
		fmt.Printf("Error! at %s no test with if statement at AST node id %d\n", _file_line_(),ifNode.id)
		return nil 
	}  else {
		testStmt.ifRoot = ifStmt
	}

	if (takenStmt == nil) {
		fmt.Printf("Error! at %s no taken block with if statement at AST node id %d\n", _file_line_(),ifNode.id)
		return nil 
	} else {
		// takenStmt.ifRoot = ifStmt 
	}


	// santiy check, both the else an sub if statement can not be set 
	if (elseStmt != nil) && (subIfStmt != nil) {
		fmt.Printf("Error! at %s both else and if sub-statement set AST node id %d\n", _file_line_(),ifNode.id)
		fmt.Printf("Error! at %s statement len %d %s\n",_file_line_(),len(statements),statements)
		//elseStmt.ifRoot = ifStmt 		
	}

	// if we passed the above sanity check/assertion, we can set the sub-if statement 
	if (subIfStmt != nil) {
		elseStmt = elseHead 
		ifStmt.ifElse = subIfStmt
		testStmt.addStmtSuccessor(subIfStmt)
		subIfStmt.addStmtSuccessor(eosStmt)
		subIfStmt.addStmtPredecessor(ifStmt)		
		eosStmt.addStmtPredecessor(subIfStmt)
		//subIfStmt.ifRoot = ifStmt 
	} else if (elseStmt != nil) {
		ifStmt.ifElse = elseStmt

	}

	// This sets the pred and successor links for the simple statement 
	if (simpleStmt != nil) {
				
		statements = append(statements,simpleStmt)

		simpleStmt.addStmtSuccessor(testStmt)
		testStmt.addStmtPredecessor(simpleStmt)
		simpleStmt.ifRoot = ifStmt 
	}

	// there must always be a test and taken statement
	statements = append(statements,testStmt)
	testStmt.addStmtSuccessor(takenStmt)
	takenStmt.addStmtPredecessor(testStmt)
	//takenStmt.addStmtSuccessor(eosStmt)
	//takenStmt.ifRoot = ifStmt 

	statements = append(statements,takenStmt)

	if (takenTail != nil) {
		takenTail.addStmtSuccessor(eosStmt)		
	}

	// the else and subIf are interchangable as the 2nd clause 
	if (elseStmt != nil) && (subIfStmt == nil) {
		testStmt.addStmtSuccessor(elseStmt)
		elseStmt.addStmtPredecessor(testStmt)
		elseTail.addStmtSuccessor(eosStmt)
	} 

	if (takenHead != nil) || (takenTail != nil) || (elseHead != nil) || (elseTail !=nil) {
		//fmt.Printf("IF statement at %s statement len %d \n",_file_line_(),len(statements))
		Pass()
	}
	
	return statements 
}

// This parses a for statement.
// It tried to get the block and forClause first. Then it walks the children of the for clause and creates new
// statement nodes as it walks the forClause. The end of the function creates the edges between the statement nodes 
func (l *argoListener) parseForStmt(forNode *ParseNode,funcDecl *ParseNode,forStmt,eosStmt *StatementNode) []*StatementNode {
	var funcName *ParseNode               // name of the function
	var funcStr  string                 // name of the function
	var forClauseNode  *ParseNode              //  if this statement has a for clause
	var forBlockNode   *ParseNode             // the block of statements for the for
	var subNode        *ParseNode         // sub-simple statement type
	var statements []*StatementNode       // list of statmements 
	var blocklist []*StatementNode          // list of statements for the block

	var childStmt *StatementNode       
	var blockStmt, initStmt, conditionStmt, postStmt  *StatementNode // statement nodes for the for statement
	var blockHead, blockTail *StatementNode   // the head and tail of the block statement 
	var seenSimple int
	
	statements = nil
	blocklist = nil
	
	// get the name of the function the ifStmt is in  
	funcName = funcDecl.children[1]
	funcStr = funcName.sourceCode // ToDO: change this to use the ruleType instead of the source code 

	// loop for each child and create the appropriate sub-statement node
	// after looping through all the children, we fix up the successors and predecessors edges
	forStmt.visited = true
	forClauseNode = nil
	forBlockNode = nil
	blockStmt = nil
	seenSimple = 0  // how many simple statements have we seen?

	// walk the for statement clauses
	// create a new statement node, except for the block statement 
	for _, childNode := range forNode.children {

		// create a new statement node 
		if (childNode.ruleType == "expression") {
			childStmt = new(StatementNode)
			childStmt.id = l.nextStatementID; l.nextStatementID++ // create new ID
			childStmt.parseDef = childNode
			childStmt.parseDefID =  childNode.id 
			childStmt.funcName = funcStr
			childStmt.sourceRow =  childNode.sourceLineStart
			childStmt.sourceCol =  childNode.sourceColStart
			childStmt.parent = forStmt 
			childStmt.parentID = forStmt.id

			// set the variable scope to be the same as the for statement's scope 
			childStmt.vScope = forStmt.vScope 
			childStmt.vScope.statements = append(childStmt.vScope.statements,childStmt)

			l.statementGraph = append(l.statementGraph,childStmt)

			conditionStmt = childStmt
			subNode =  childNode.children[0]
			conditionStmt.stmtType = subNode.ruleType
			conditionStmt.parseSubDef = subNode 
			conditionStmt.parseSubDefID =  subNode.id
			
		}
		if (childNode.ruleType == "block") {
			forBlockNode = childNode
			
		}

		if (childNode.ruleType == "forClause") {
			forClauseNode = childNode 
		}

	} 

	// if we have a forClause, walk these children 
	if (forClauseNode != nil) { 
		for _, childNode := range forClauseNode.children {
			
			// note the child statement can be nil around this loop if the type is not one of
			// simpleStmt, expression, block of ifStmt
			if (childNode.visited == false) {

				if (childNode.ruleType == "block") {
					forBlockNode = childNode
					
				}
				
				// create the new node 
				if (childNode.ruleType == "expression") || (childNode.ruleType == "simpleStmt")  {
					childStmt = new(StatementNode)
					childStmt.id = l.nextStatementID; l.nextStatementID++ // create new ID
					childStmt.parseDef = childNode
					childStmt.parseDefID =  childNode.id 
					childStmt.funcName = funcStr
					childStmt.sourceRow =  childNode.sourceLineStart
					childStmt.sourceCol =  childNode.sourceColStart
					childStmt.parent = forStmt 
					childStmt.parentID = forStmt.id

					// set the variable scope to be the same as the if statement's scope 
					childStmt.vScope = forStmt.vScope 
					childStmt.vScope.statements = append(childStmt.vScope.statements,childStmt)
					
					// add this to the global list of statements 
					l.statementGraph = append(l.statementGraph,childStmt)
					
				}
				
				if (childNode.ruleType == "expression") {
					conditionStmt = childStmt
					subNode =  childNode.children[0]
					conditionStmt.stmtType = subNode.ruleType
					conditionStmt.parseSubDef = subNode 
					conditionStmt.parseSubDefID =  subNode.id
				}

				// the second simple statement is the post-condition 
				if (seenSimple == 1) && (childNode.ruleType == "simpleStmt")  {
					postStmt = childStmt

					subNode =  childNode.children[0]
					postStmt.stmtType = subNode.ruleType
					postStmt.parseSubDef = subNode 
					postStmt.parseSubDefID =  subNode.id

					seenSimple++
				}

				// the first simple statement is the initialization statement
				// add check here if there is a short var decl; create a new
				// var context for this statement 
				if (seenSimple == 0 ) && (childNode.ruleType == "simpleStmt")  {
					initStmt = childStmt

					subNode =  childNode.children[0]
					initStmt.stmtType = subNode.ruleType
					initStmt.parseSubDef = subNode 
					initStmt.parseSubDefID =  subNode.id

					seenSimple++ 
				}
				

			} // end if not visited 
		} // end of forcause children loop
	} // end if for clause != nil 

	if (forBlockNode != nil) {
		statementListNode := forBlockNode.children[1] // get the list of statements in the block 
		blocklist = l.getListOfStatements(statementListNode,forStmt,funcDecl)

		// get the list of statements from the statementlist 
		if (len(blocklist) >0)  {

			// connect the taken of else clause back to this node
			blockHead = blocklist[0]
			blockTail = blocklist[len(blocklist)-1] 
			blockStmt = blockHead 
		} else {
			// should not happen, zero length block
			fmt.Printf("error, for statement with zero statement block \n")
		}
		
	}

	// add links to the main if statement for the control flow graph 
	forStmt.forInit = initStmt
	forStmt.forCond = conditionStmt
	forStmt.forPost = postStmt 
	forStmt.forBlock = blockStmt
	forStmt.forTail = blockTail
	
	// This sets the pred and successor links for the initialization statement
	// note this assume the block is defined, which is might not be
	
	if (initStmt != nil) {
		statements = append(statements,initStmt)
		if ( conditionStmt != nil) {
			initStmt.addStmtSuccessor(conditionStmt)
			conditionStmt.addStmtPredecessor(initStmt)
		}
		initStmt.forRoot = forStmt 
	}

	if (conditionStmt != nil) {
		statements = append(statements,conditionStmt)
		// there must always be a block statement 
		conditionStmt.addStmtSuccessor(blockHead)
		blockHead.addStmtPredecessor(conditionStmt)
		conditionStmt.forRoot = forStmt 
	} else {
		blockHead.addStmtPredecessor(forStmt)		
	}
	
	if (len(blocklist) > 0) {
		statements = append(statements,blocklist...)
	}
	
	if (postStmt != nil) {
		statements = append(statements,postStmt)
		postStmt.forRoot = forStmt 
		if (len(blocklist) > 0) {
			postStmt.addStmtPredecessor(blockTail)
			blockTail.addStmtSuccessor(postStmt)
		}
		if (conditionStmt != nil) {
			postStmt.addStmtSuccessor(conditionStmt)
		}

	} else {
		if (conditionStmt != nil) {
			blockTail.addStmtSuccessor(conditionStmt)
			
		} else {
			// an if statement tail should go to a fake post statement
			// try pointing back to the for node in this case 
			blockTail.addStmtSuccessor(forStmt)
		}
	}

	
	// if (len(statements) > 0) {
	//   statements[len(statements)-1].setStmtPredNil()
	//  }
	
	return statements 
}

func (l *argoListener) parseSwitchStmt(switchnode *ParseNode,funcDecl *ParseNode) [][]*StatementNode {
	return nil
}


func (l *argoListener) parseSelectStmt(selectnode *ParseNode,funcDecl *ParseNode) [][]*StatementNode {
	return nil
}


// create a return variable node. When a return happens, we store the value
// in this generated variable and treat the return as an expression with
// additional control flow 
func (l *argoListener) makeReturnVariable(identifierR_type *ParseNode,funcName string) *VariableNode {
	var varTypeStr string
	var numBits int 
	var retVarNode *VariableNode

	retVarNode = nil
	varTypeStr,numBits = identifierR_type.getPrimitiveType()
	
	if (varTypeStr != "") {
		retVarNode = new (VariableNode)
		retVarNode.id = l.nextVarID ; l.nextVarID++		
		
		lineStartStr := strconv.Itoa(identifierR_type.sourceLineStart)
		colStartStr := strconv.Itoa(identifierR_type.sourceColStart)
		
		retVarNode.parseDef = identifierR_type 
		retVarNode.parseDefNum = identifierR_type.id 
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
	var funcName *ParseNode    // name of the function -- AST node 
	var funcStr string       // name of the function as a string. Must be unique
	var resultNode *ParseNode  // node for the result 
	var retParams *ParseNode    // the parameters for the return values for a function call
	var identifierR_type *ParseNode  // node for getting the primitive type 
	var fNode *FunctionNode   // node of the function we are creating 

	var retVarNode *VariableNode // the variable node for the return value 
	
	// get parameters assumes we have the variables already parsed
	if (len(l.varNodeList) <= 0) {
		fmt.Printf("Error: at %s, warning no variables in getallfunctions\n",_file_line_())
	}

	for i, funcDecl := range l.ParseNodeList {
		if (funcDecl.ruleType == "functionDecl") {
			if (len(funcDecl.children) < 2) {  // need assertions here 
				fmt.Printf("Error at %s: %d no function name",_file_line_(),i)
			}
			funcName = funcDecl.children[1]
			funcStr = funcName.ruleType
			if (len(funcStr) > 0) {
				fNode = new(FunctionNode)
				fNode.id = l.nextFuncID; l.nextFuncID++
				fNode.funcName = funcStr
				fNode.sourceRow = funcDecl.sourceLineStart
				fNode.sourceCol = funcDecl.sourceColStart
				l.funcNodeList = append(l.funcNodeList,fNode)
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
						fNode.retVars = append(fNode.retVars,retVarNode)
						fNode.retVarsIDs = append(fNode.retVarsIDs,retVarNode.id)
					} else { // we have a parameter list 
						for _, typeNode := range retParams.children {
							identifierR_type = typeNode.walkDownToRule("r_type")
							if (identifierR_type == nil) { continue }  // skip 
							retVarNode =  l.makeReturnVariable(identifierR_type,funcStr)
							if (retVarNode == nil) {
								fmt.Printf("Error making return var node\n")
							} else {
								fNode.retVars = append(fNode.retVars,retVarNode)
								fNode.retVarsIDs = append(fNode.retVarsIDs,retVarNode.id)
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


// Given a statementlist, return a list of StatementNodes
// Uses recursion to follow if and for, case and select statements
// Evey list must end with an End-Of-Statement (EOS) which is the terminal node for this list and
// Past as the terminal statement for various sub-statements
func (l *argoListener) getListOfStatements(listnode *ParseNode,parentStmt *StatementNode,funcDecl *ParseNode) []*StatementNode {
	var funcName *ParseNode  // name of the function for the current statement
	var funcStr  string   //  string name of the function
	var numChildren,i int 
	var subNode *ParseNode  // current statement node
	var eosNode *ParseNode  // the end of the statement list node
	var eosStmt *StatementNode  // the node of the end-of-statement node 
	var stmtTypeNode *ParseNode // which simpleStmt type is this?, e.g ifStmt, shortVarDecl, forStmt.
	var statementList []*StatementNode
	var predecessorStmt *StatementNode 
	var stateNode *StatementNode
	var slist []*StatementNode
	var varDeclList []*VariableNode // a list of variables for a declaration node 
	//var numChildren int
	
	if (len(funcDecl.children) < 2) {  // need assertions here 
		fmt.Printf("Major Error")
	}
	funcName = funcDecl.children[1]
	funcStr = funcName.sourceCode

	predecessorStmt = nil


	// every statement is separated by and EOS statement separator
	// This loop advances through the list in groups of 2, one for the main statement and one for the eos 
	// We will need to pass the eos to the recursively defined statements such as if, for and case
	// that have a block/list as a sub-statement 
	numChildren = len(listnode.children)
	if ( (numChildren % 2) !=0 ) { // check we have an even number of children as every statement has and eos separator 
		fmt.Printf("Error: got statement list with odd number of statements %d \n",_file_line_())
		return nil
	}

	// here is the main loop walking down a list of statements 
	for i =0; i < (numChildren-1) ; i=i+2 {

		childNode := listnode.children[i]
		if (childNode.visited == true) {
			continue 
		}
		childNode.visited = true

		
		// check the type if we want to continue 
		if (len(childNode.children) > 0) {
			subNode = childNode.children[0] // subnode should be a statement
			
			// skip decls 
			if (subNode.ruleType != "declaration" )&& (subNode.ruleType != ";") && (len(subNode.children) >0) {
				// simple statements have to go one level down to get the actual type 
				if (subNode.ruleType == "simpleStmt") { 
					stmtTypeNode = subNode.children[0]
				} else { 
					stmtTypeNode = subNode
				}
			} else {
				if (subNode.ruleType == "declaration") {
					varDeclList = l.getParseVariables(subNode.children[0])
					// fmt.Printf("got a declaration %d %d \n",subNode.id,len(varDeclList))
					
				} else if (subNode.ruleType == "shortVarDecl")  {
					varDeclList = l.getParseVariables(subNode)
					//fmt.Printf("got other declaration %d %d \n",subNode.id,len(varDeclList))
			}

				continue;

			}

		} else {
			continue;
		}

		// create the new statement node 
		stateNode = new(StatementNode)
		stateNode.id = l.nextStatementID; l.nextStatementID++
		stateNode.parseDef = childNode
		stateNode.parseDefID =  childNode.id
		stateNode.parseSubDef = stmtTypeNode
		stateNode.parseSubDefID =  stmtTypeNode.id
		stateNode.stmtType = stmtTypeNode.ruleType
		stateNode.funcName = funcStr
		stateNode.sourceRow =  stmtTypeNode.sourceLineStart 
		stateNode.sourceCol =  stmtTypeNode.sourceColStart
		stateNode.parent = parentStmt
		stateNode.parentID = parentStmt.id
		stateNode.child = nil
		stateNode.childID = -1
		stateNode.ifSimple = nil 
		stateNode.ifTaken = nil
		stateNode.ifElse  = nil
		stateNode.forInit = nil
		stateNode.forCond = nil 
		stateNode.forPost = nil
		stateNode.forBlock = nil
		stateNode.caseList = nil
		// append to the local and global lists of statements 
		statementList = append(statementList,stateNode)	 // local list 					
		l.statementGraph = append(l.statementGraph,stateNode) // global list 
		
		// create the eos statement node, we will need it later
		// The eos is the exit point for a sub list/block 
		eosNode = listnode.children[i+1]
		eosNode.visited = true

		eosStmt = new(StatementNode)
		eosStmt.id = l.nextStatementID; l.nextStatementID++
		eosStmt.parseDef = eosNode
		eosStmt.parseDefID =  eosNode.id
		eosStmt.parseSubDef = eosNode
		eosStmt.parseSubDefID =  eosNode.id
		eosStmt.stmtType = eosNode.ruleType
		eosStmt.funcName = funcStr
		eosStmt.sourceRow =  eosNode.sourceLineStart 
		eosStmt.sourceCol =  eosNode.sourceColStart
		eosStmt.parent = parentStmt
		eosStmt.parentID = parentStmt.id
		eosStmt.child = nil
		eosStmt.childID = -1
		eosStmt.ifSimple = nil 
		eosStmt.ifTaken = nil
		eosStmt.ifElse  = nil
		eosStmt.forInit = nil
		eosStmt.forCond = nil 
		eosStmt.forPost = nil
		eosStmt.forBlock = nil
		eosStmt.caseList = nil

		statementList = append(statementList,eosStmt)						
		l.statementGraph = append(l.statementGraph,eosStmt)
		
		// attach the predecessor to the newly generated node
		if (predecessorStmt != nil) {
			predecessorStmt.addStmtSuccessor(stateNode)
		}
		stateNode.addStmtSuccessor(eosStmt)
		stateNode.addStmtPredecessor(predecessorStmt)
		eosStmt.addStmtPredecessor(stateNode)

		
		// this added to the scope of the statement
		// inheret the variable scope from either the predecessor or the	
	// parent statement 
		if (predecessorStmt != nil) {
			if (predecessorStmt.vScope != nil) {
				stateNode.vScope = predecessorStmt.vScope
				eosStmt.vScope = predecessorStmt.vScope
			} else {
				if (parentStmt.vScope != nil ) {
					stateNode.vScope = parentStmt.vScope
					eosStmt.vScope = parentStmt.vScope
				} else {
					fmt.Printf("Error at %s no variable scope stmt id %d\n",_file_line_(),stateNode.id)
				}
			}
		} else {
			if (parentStmt.vScope != nil ) {
				stateNode.vScope = parentStmt.vScope
				eosStmt.vScope = parentStmt.vScope
			} else {
				fmt.Printf("Error at %s no variable scope stmt id %d\n",_file_line_(),stateNode.id)
			}				
		}


		stateNode.vScope = parentStmt.vScope
		stateNode.vScope.statements = append(stateNode.vScope.statements,stateNode)
		
		eosStmt.vScope = parentStmt.vScope
		eosStmt.vScope.statements = append(eosStmt.vScope.statements,eosStmt)
		
		// add variables to the scope for this statement
		// only adds statements with a var keyword in front. 
		for _, vNode := range(varDeclList) {
			//fmt.Printf("stmt: %d adding declaration var %s \n",stateNode.id,vNode.sourceName)
			// find the variable name in the global list of vatiables.
			stateNode.vScope.varNameMap[vNode.sourceName] = vNode
			// once the node is added, clear the list from the loop
			varDeclList = nil
		}
	
		// Get sub statement lists for this node
		switch stateNode.stmtType { 
		case "declaration":
			// add the variable to the scope context
			fmt.Printf("got declaration %d \n",stateNode.id)
		case "labeledStmt":
			
		case "goStmt":
		case "returnStmt":
		case "breakStmt":
		case "continueStmt":
		case "gotoStmt":
		case "fallthroughStmt":
		case "ifStmt":
			// get the eos of the next node
			// this will become the successor in the statement graph
			// create a new variable scope for this statement
			
			slist = l.parseIfStmt(subNode,funcDecl,stateNode,eosStmt)
			slistLen := len(slist)
			if (slistLen > 0) {
				stateNode.child = slist[0]
				stateNode.childID = slist[0].id

				// note the successor of the last statment in the
				// sub-block is the eos of the if statement 
				//slist[slistLen-1].addStmtSuccessor(eosStmt)
			}
						
		case "switchStmt":
		case "selectStmt":
		case "forStmt":
			// create a new variable scope for this statement
			slist = l.parseForStmt(subNode,funcDecl,stateNode,eosStmt)
			slistLen := len(slist)			
			if (slistLen > 0) {
				stateNode.child = slist[0]
				stateNode.childID = slist[0].id
				//slist[slistLen-1].addStmtSuccessor(eosStmt)				
			}
						
		case "sendStmt":
		case "expressionStmt":
		case "incDecStmt":
		case "assignment":
		case "shortVarDecl":
			fmt.Printf("got short var decl %d \n",stateNode.id)
		case "emptyStmt":
						
		default:
			fmt.Printf("Error! at %s no such statement type: %s\n",_file_line_(),stateNode.stmtType)
		}

		predecessorStmt = eosStmt
	} // end for loop of statementlist 
	
	if (statementList == nil) {
		fmt.Printf("Error! return from statementlist has zero statements %s\n", _file_line_())
		return nil 
	}
	
	return statementList 
}

// fix the edges of the return statements to their successors to the exit nodes of the function/
// this assumes the function declaration is linearly ordered in the statementgraph with the function
// calls. If not, we need a walkUpToRule call. Assuming linear ordering for now.
func (l *argoListener) addInternalReturnEdges() {
	var functionEntry, functionExit *StatementNode
	// find the exit node

	for _, stmtNode := range(l.statementGraph) {
		stmtNode.visited = false 
	}
	
	functionExit = nil 
	for _, stmtNode := range(l.statementGraph) {

		if (stmtNode.stmtType == "functionDecl") {
			functionEntry = stmtNode 
			functionExit = functionEntry.successors[0]
		}

		if (stmtNode.stmtType == "returnStmt") {
			if (functionExit != nil) {
				stmtNode.setStmtSuccNil()
				stmtNode.addStmtSuccessor(functionExit)
				// make sure the function exit gets the return node as a predecessors
				functionExit.addStmtPredecessor(stmtNode)
			}else {
				fmt.Printf("Error! return edges called with mil function parent %s\n", _file_line_())
			}
		}
	}
}

// remove the predecessor edges for the EOS nodes
// then add them back in based on the successor edges
func (l *argoListener) fixEosPredecessors() {

	// remove all the predecessor edges 
	for _, stmtNode := range(l.statementGraph) {
		if stmtNode.stmtType == "eos" {
			stmtNode.predecessors = nil
			stmtNode.predIDs = nil 
		}
	}

	// if we found an EOS as a target, add a predecessor edge 
	for _, stmtNode := range(l.statementGraph) {
		if len(stmtNode.successors) > 0 {
			succNode := stmtNode.successors[0]
			if succNode.stmtType == "eos" {
				succNode.addStmtPredecessor(stmtNode)
			}
		}
	}

}

// get a function statement Node by the functions name 
func (l *argoListener) getFunctionStmtEntry(funcName string) *StatementNode {
	for _, stmtNode := range(l.statementGraph) {
		stmtNode.visited = false

		if (stmtNode.stmtType == "functionDecl") {
			if (stmtNode.funcName == funcName) {
				return stmtNode
			}
		}
	}
	return nil 
}


// add edges to the caller 
func (l *argoListener) addCallandReturnEdges() {
	var funcEntryNode,functionExitNode *StatementNode
	var retList []*ParseNode
	var parentNode, operandNameNode *ParseNode
	var calleeNameStr string

	for _, stmtNode := range(l.statementGraph) {
		stmtNode.visited = false 
	}

	// these statement types may have a function call 
	for _, stmtNode := range(l.statementGraph) {
		// statement types which may make a function call 
		if ((stmtNode.stmtType == "expression") || (stmtNode.stmtType == "assignment") ||
			(stmtNode.stmtType == "shortVarDecl") || (stmtNode.stmtType == "expressionStmt") ||
			(stmtNode.stmtType == "unaryExpr")  || (stmtNode.stmtType == "returnStmt") ||
		        (stmtNode.stmtType == "goStmt")) {

			// if something in the AST has parameters, we declare it having at least one functio
			retList = stmtNode.parseDef.walkDownToAllRules("arguments")

			// check if we have multiple calls in this statement. If so we walk the list of
			// called functions and add a successor edge in the statementgraph for each one 
			for _, argNode := range retList {
				parentNode = argNode.parent
				operandNameNode = parentNode.walkDownToRule("operandName")
				if (operandNameNode != nil) {
					calleeNameStr = operandNameNode.children[0].ruleType 
					// find the functionDecl node with this name
					funcEntryNode = l.getFunctionStmtEntry(calleeNameStr)
					// if we can't find the function, then abort this node 
					if (funcEntryNode != nil) {
						// add predecessor to the function entry node 
						// funcEntryNode.addStmtPredecessor(stmtNode)
						funcEntryNode.callers = append(funcEntryNode.callers,stmtNode)
						// non-go statements add a return edge
						if (stmtNode.stmtType != "goStmt") {
							stmtNode.callTargets=append(stmtNode.callTargets,funcEntryNode)
							functionExitNode = funcEntryNode.successors[0]
							functionExitNode.returnTargets = append(functionExitNode.returnTargets,stmtNode)
							
						} else {
							stmtNode.goTargets=append(stmtNode.goTargets,funcEntryNode)
						}
					} else {
						// check if a fmt.printf, make or cast statements. 
						// need to add check if this is a printf statement 
						// fmt.Printf("function %s not found \n",calleeNameStr)
					}
					
				} else {
					fmt.Printf("Error! no operandNode at %d\n", _file_line_())			
				}
			}

		}
	}
}


// convert the right hand side of an expression to a string 
func (l *argoListener) rightHandSideStr() {
	
}
	
// for assignment and short var decls, add the left and right hand sides of the assignment expression
func (l *argoListener) addVarAssignments() {
	var funcStr string
	var varStrList []string 
	var parsedNode, funcParseNode, funcNameNode,lhsNode,operandNameNode *ParseNode
	var operandNameNodeList []*ParseNode 
	var varNode *VariableNode
	var funcNode *FunctionNode
	var varStr string
	
	for _, stmtNode := range(l.statementGraph) {
		stmtNode.visited = false 
	}

	// these statement types may have a variable assignment 
	for _, stmtNode := range(l.statementGraph) {


		// get the function header 
		if ((stmtNode.stmtType == "assignment") || (stmtNode.stmtType == "shortVarDecl") || 
			(stmtNode.stmtType == "sendStmt") || (stmtNode.stmtType == "funcDecl") ||
			(stmtNode.stmtType == "returnStmt")) {
			
			parsedNode = stmtNode.parseSubDef
			funcParseNode = parsedNode.walkUpToRule("functionDecl")
			funcNameNode = funcParseNode.children[1]
			funcStr = funcNameNode.ruleType

			operandNameNodeList = make([]*ParseNode,0)
			varStrList = make([]string,0)
		}
		// statement types which may have an assignment
		if ((stmtNode.stmtType == "assignment") || (stmtNode.stmtType == "shortVarDecl") || 
			(stmtNode.stmtType == "sendStmt")) {

			// get the left hand side of the statement 

			parsedNode = stmtNode.parseSubDef
			lhsNode = parsedNode.children[0]
			
			funcParseNode = parsedNode.walkUpToRule("functionDecl")
			funcNameNode = funcParseNode.children[1]
			funcStr = funcNameNode.ruleType

			// make sure we get all the variables in the assignment for
			// when there are multiple ones in a return statement
			if (stmtNode.stmtType == "assignment") {
					operandNameNodeList = lhsNode.walkDownToAllRules("operandName")
				for _, opNode := range(operandNameNodeList) {
					varStrList = append(varStrList,opNode.children[0].ruleType)
				}
				
			}

			// channels cannot accept mulitple values 	
			if (stmtNode.stmtType == "sendStmt") { 
				operandNameNode = parsedNode.walkDownToRule("operandName")
				varStrList = append(varStrList,operandNameNode.children[0].ruleType)
			}

			// TODO: need to fix this to be able to return multiple values for a short vardecl
			// that returns multiple values. 
			if (stmtNode.stmtType == "shortVarDecl")  { 
				operandNameNode = parsedNode.walkDownToRule("identifierList")
				varStrList = append(varStrList,operandNameNode.children[0].ruleType)
			}

			// parsedNode.sourceLineStart,parsedNode.sourceColStart,varStr)

			// iterate through the variables names and and them to the LHS expression 
			for _, varStr = range(varStrList) {
				varNode = l.getVarNodeByNames("",funcStr,varStr)
				if (varNode == nil) {
					fmt.Printf("Error!, at %d no variable func %s name %s\n",_file_line_(),funcStr,varStr)
					continue
				}
				stmtNode.writeVars = append(stmtNode.writeVars,varNode)
			}


			// get the expression on the right hand side 
		}

		// function entry copies the parameters
		// function return copies the RHS into the list of return values 
		if ( (stmtNode.stmtType == "funcDecl") || (stmtNode.stmtType == "returnStmt")) {
			
			funcNode = l.getFuncNodeByNames("",funcStr)
			if (funcNode == nil) {
				fmt.Printf("Error! at %d no func %s name \n",_file_line_(),funcStr)
				continue
			}

			if (stmtNode.stmtType == "funcDecl") {
				for _, funcVar := range(funcNode.parameters) {
					stmtNode.writeVars = append(stmtNode.writeVars,funcVar)
				}
			}
			
			if (stmtNode.stmtType == "returnStmt") {
				for _, retVar := range(funcNode.retVars) {
					stmtNode.writeVars = append(stmtNode.writeVars,retVar)
				}				
			}
		}

	} // end for all statements 
}

// Generate a control flow graph (CFG) at the statement level.
// We look for statement lists. If we find one, back up to the
// enclosing function to find the function def to use as and entry
// point. Then recursively decend down the statement list
// Assumes there is only one statement list per function (fixme)

func (l *argoListener) getStatementGraph() int {
	var sourceFile *ParseNode // source file high level node
	var funcDecl *ParseNode  // Function Declaration 
	var funcName *ParseNode  // name of the function for the current statement
	var blockNode *ParseNode
	var stmtListNode *ParseNode   // the statement node
	var funcEOS  *ParseNode // exit node of the function 
	var funcStr  string   //  string name of the function
	
	var startNode, entryNode,exitNode,finishNode *StatementNode // the entry node and end nodes in the entire graph
	var statements []*StatementNode // list of statement nodes

	
	// mark all nodes as not visited 
	for _, node := range l.ParseNodeList {
		node.visited = false
	}

	// find the high level function declarations in the source file
	sourceFile = l.ParseNodeList[0]
	if (sourceFile.ruleType != "SourceFile") {
		fmt.Printf("Error! first AST node not a SourceFile\n")
		return 0 
	}


	// the start node is a special root node to begin the execution of the
	// system. We look for main() function which is the successor of the start statement node
	startNode = new(StatementNode)
	startNode.id = l.nextStatementID; l.nextStatementID++
	startNode.parseDef = nil
	startNode.parseDefID =  0
	startNode.parseSubDef = nil 
	startNode.parseSubDefID =  0
	startNode.stmtType = "startNode"
	startNode.funcName = "__start__"
	startNode.sourceRow =  0
	startNode.sourceCol =  0
	startNode.parent = nil
	startNode.parentID = -1
	startNode.vScope = nil
	l.statementGraph = append(l.statementGraph,startNode)
	
	// for each function in the source file, parse the linear list of statements 
	for i, childNode := range sourceFile.children {

		// pull out the function entry and exit points. 
		if (childNode.ruleType == "functionDecl") {

			// this is the End-of-String (EOS) of the function, which is the exit point
			funcDecl = childNode 
			funcEOS = sourceFile.children[i+1]
				
			if (len(funcDecl.children) < 2) {  // need assertions here 
				fmt.Printf("Error at %d, functionDecl not enough children\n",_file_line_())
			}
			
			funcName = funcDecl.children[1]
			funcStr = funcName.ruleType // RPM-was sourcecode 

			// add the function declaration to the statement graph as the
			// entry point for the statements in the function
			// this entry point becomes the copy-in for parameters in the block-graph
			blockNode = funcDecl.walkDownToRule("block")
			if (blockNode == nil) {
				fmt.Printf("Error at %d, no block in function %s\n",_file_line_(),funcStr)
				continue 
			}
			stmtListNode = blockNode.walkDownToRule("statementList")
			if (stmtListNode == nil) {
				fmt.Printf("Error at %d, no statement list in function %s\n",_file_line_(),funcStr)
				continue 
			}			

			// We need to include function declarations in the statement graph because
			// they are the place node of copying in the arguments in the graph.
			if (funcDecl.visited == false ) {
				funcDecl.visited = true

				entryNode = new(StatementNode)
				entryNode.id = l.nextStatementID; l.nextStatementID++
				entryNode.parseDef = funcDecl
				entryNode.parseDefID =  funcDecl.id
				entryNode.parseSubDef = nil 
				entryNode.parseSubDefID =  0
				entryNode.stmtType = funcDecl.ruleType
				entryNode.funcName = funcStr
				entryNode.sourceRow =  funcDecl.sourceLineStart 
				entryNode.sourceCol =  funcDecl.sourceColStart
				entryNode.parent = nil
				entryNode.parentID = -1
				entryNode.vScope = new(VarScope)
				entryNode.vScope.varNameMap = make(map[string] *VariableNode)
				entryNode.vScope.id = entryNode.id
				entryNode.vScope.statements = append(entryNode.vScope.statements,entryNode)
				// add the variables in the function declaration to the scope
				// if the function and parameter fields match, add to the
				// scope 
				for _, node := range l.varNodeList {
					if (node.funcName == funcStr) {
						if (node.isParameter) {
							entryNode.vScope.varNameMap[node.sourceName]=node
							entryNode.vScope.statements = append(entryNode.vScope.statements,entryNode)
						}
					}
				}
				
				// add this to the global list of statements 
				l.statementGraph = append(l.statementGraph,entryNode)
				
				statements = make([]*StatementNode,1)
				statements = l.getListOfStatements(stmtListNode,entryNode,funcDecl)

				// if this is the main function, make the predicessor the startNode and
				// the successor of the start node the main() definition
				// else the predecessors 
				if (funcStr == "main") {
					startNode.child = entryNode
					startNode.childID = entryNode.id
					entryNode.parent = startNode
					startNode.addStmtSuccessor(entryNode)
					entryNode.addStmtPredecessor(startNode)
				} 
					
				exitNode = new(StatementNode)
				exitNode.id = l.nextStatementID; l.nextStatementID++
				exitNode.parseDef = funcEOS
				exitNode.parseDefID =  funcEOS.id
				exitNode.parseSubDef = nil 
				exitNode.parseSubDefID =  0
				exitNode.stmtType = "FuncExit"
				exitNode.funcName = funcStr
				exitNode.sourceRow =  funcEOS.sourceLineStart 
				exitNode.sourceCol =  funcEOS.sourceColStart
				exitNode.parent = entryNode
				exitNode.parentID = entryNode.id
				exitNode.child = nil
				exitNode.childID = -1

				exitNode.vScope = entryNode.vScope
				exitNode.vScope.statements = append(exitNode.vScope.statements,exitNode)
				// add this to the global list of statements 
				l.statementGraph = append(l.statementGraph,exitNode)

				if (funcStr == "main") {
					finishNode = new(StatementNode)
					finishNode.id = l.nextStatementID; l.nextStatementID++
					finishNode.parseDef = nil
					finishNode.parseDefID =  0
					finishNode.parseSubDef = nil 
					finishNode.parseSubDefID =  0
					finishNode.stmtType = "finishNode"
					finishNode.funcName = "__finish__"
					finishNode.sourceRow =  exitNode.sourceRow + 1
					finishNode.sourceCol =  0
					finishNode.parent = nil
					finishNode.parentID = -1
					l.statementGraph = append(l.statementGraph,finishNode)

					exitNode.addStmtSuccessor(finishNode)
					finishNode.addStmtPredecessor(exitNode)
				} 

				if (len(statements) > 0) {
					exitNode.addStmtPredecessor(statements[len(statements)-1])
				}
				
				// fix the predecessor/successor edges 
				entryNode.addStmtSuccessor(exitNode)
				// note the predecessor of an exit nodes are the return statements
				// exitNode.addStmtPredecessor(entryNode)
				
				// set the edges from the entry point to this list
				
				if ( len(statements) > 0) {
					entryNode.child = statements[0]
					entryNode.childID = statements[0].id
					// make sure to add the funcDecl as a predecessor
					entryNode.child.addStmtPredecessor(entryNode)
					lastNode := statements[len(statements)-1]
					lastNode.addStmtSuccessor(exitNode)
				} else {
					fmt.Printf("Warning: Function %s has zero statements",funcStr)
				}


			} else {
				//fmt.Printf("Graph: AST Node %d was visited \n",astnode.id)
				Pass()
			}
			
		}
	}

	// because linkdanges is recursive, we have these outer loops 
	for _, stmtNode := range(l.statementGraph) {
		stmtNode.visited = false 
	}

	// Remove dangling pointers by adding return edges 
	for _, stmtNode := range(l.statementGraph) {
		if (stmtNode.stmtType == "functionDecl") {
			// l.linkDangles(stmtNode,stmtNode.successors[0])
		}
	}

	// adding return edges is not recursize, so we have a flat call 
	for _, stmtNode := range(l.statementGraph) {
		stmtNode.visited = false 
	}

	// fix the target end-of-statements predecessors 
	l.fixEosPredecessors() 

	// fix up various edges
	l.addInternalReturnEdges()
	// Add call and return edges
	l.addCallandReturnEdges()

	// start the data flow section with the assignments 
	l.addVarAssignments()
		
	return 1
} // end getStatementGraph 


func (l *argoListener) generateNewScope(stmt *StatementNode) {
	var pNode *ParseNode

	pNode = stmt.parseSubDef
	// fmt.Printf("  Generate New Scope called stmt %d parse %d \n",stmt.id,pNode.id)
	
	// match the variable to this parse node 
	for _, vNode := range l.varNodeList {
		// fmt.Printf(" parsedef is %d \n",vNode.parseDef.id)
		if (vNode.parseDef.id == pNode.id) {
			// fmt.Printf("matched short var decl stmt %d\n",stmt.id)
			stmt.vScope.varNameMap[vNode.sourceName] = vNode
			stmt.vScope.id++;
		}
	}

}

// do a recursive traversal of the statement graph and find short var declarations
// change the variable scope nodes in the graph with the short var decls
func (l *argoListener) fixVarScopesRoot(stmt *StatementNode,pScope *VarScope)  {
	var shortDecl *StatementNode
	var newScope *VarScope
	var saveId int
	
	shortDecl = nil

	if (stmt == nil) {
		return 
	}
	if (stmt.visited == true) {
		return
	}
	stmt.visited = true

	// do a shallow copy of the parent scope into the current scope
	if (stmt.vScope == nil) {
		return
	}


	saveId = stmt.vScope.id 
	*stmt.vScope = *pScope  // make the copy
	stmt.vScope.id = saveId        // save the id
	
	// fmt.Printf("fixVarScopes copying from %d to %d \n",pScope.id,stmt.id)


	// find and replace any short var declarations in the scope rules for the statements
	if (stmt.ifSimple != nil) {
		if (stmt.ifSimple.stmtType == "shortVarDecl") {
			shortDecl = stmt.ifSimple
			l.generateNewScope(shortDecl)
			newScope = stmt.ifSimple.vScope;
			if (stmt.ifTest != nil){
				*stmt.ifTest.vScope = *newScope
			}

			// go down the taken branch 
			if (stmt.ifTaken != nil){
				*stmt.ifTaken.vScope = *newScope
				l.fixVarScopesRoot(stmt.ifTaken,newScope)
			}
			
			
			if (stmt.ifElse != nil){
				*stmt.ifElse.vScope = *newScope
				l.fixVarScopesRoot(stmt.ifElse,newScope)
			}

			
			
		}
	} else if (stmt.forInit != nil) {
		if (stmt.forInit.stmtType == "shortVarDecl") {
			shortDecl = stmt.forInit
			l.generateNewScope(shortDecl)
			newScope = stmt.ifSimple.vScope;

			if (stmt.forCond != nil){
				*stmt.forCond.vScope = *newScope
			}			

			if (stmt.forPost != nil){
				*stmt.forPost.vScope = *newScope
			}

			if (stmt.forBlock != nil){
				*stmt.forBlock.vScope = *newScope
				l.fixVarScopesRoot(stmt.forBlock,newScope)
			}			

			
		}
	}

	// this is the recursive call 
	if (stmt.child != nil) {
		l.fixVarScopesRoot(stmt.child,stmt.vScope) 
	}

	for _,succ := range (stmt.successors) {
		if (succ != nil) { 
			
			l.fixVarScopesRoot(succ,stmt.vScope)
		}
	}

}

// top-level function for fixing scope.
// go linearly through the statementgraph, then recursively fix the scope
// for each function 
func (l *argoListener) fixVariableScopes() int {

	// if this is a short var declaration, change the scope
	// traverse the rest of the graph 

	// set all nodes visited to false 
	for _, node := range l.statementGraph {
		node.visited = false; 
	}

	// recurse down the statementgraph for each function 
	for _, node := range l.statementGraph {
		if node.stmtType == "functionDecl" {
			l.fixVarScopesRoot(node,node.vScope)
		}
	}
	return 1 
}
	
/* ***************  Control Flow Graph and DataFlow Section   ********************** */

func (l *argoListener) newCFGnode(stmt *StatementNode, subID int) (int,*CfgNode) {
	var id int 
	var node *CfgNode

	id = l.nextCfgID
	l.nextCfgID++
	node = new(CfgNode)
	node.cannName = "c_bit_" + strconv.Itoa(stmt.sourceRow) + "_" + strconv.Itoa(stmt.sourceCol) + "_" +
		strconv.Itoa(subID) 
	node.id = id
	node.statement = stmt
	node.stmtID = stmt.id
	stmt.cfgNodes = append(stmt.cfgNodes,node)
	return id,node
}

// maintain order, remove from list
// update in place 
func removeCfgFromList(cNodeList []*CfgNode, nodeToRemove *CfgNode) []*CfgNode {
	for i, current := range cNodeList {
		if (current == nodeToRemove ) {
			copy(cNodeList[i:], cNodeList[i+1:]) // Shift a[i+1:] left one index.
			cNodeList[len(cNodeList)-1] = nil     // Erase last element (write zero value).
			cNodeList = cNodeList[:len(cNodeList)-1]
			return  cNodeList 
		}
	}
	return cNodeList
}


// Add a statements predecessor and successor cfg node to a cfg node 
// This is the normal linear case, where we have a linear sequence
// add the control flow node from the next statement to the CFG
// That is, appened the next cfg node to the first arg from
// the statement
// FIXME: add return error code
func addLinearToCfg(cnode *CfgNode, stmt *StatementNode) {
	var succStmt, succ *StatementNode
	succStmt = nil

	// if this is a for or if sub-statement, then skip to the for statement
	if (stmt.ifRoot != nil) {
		stmt = stmt.ifRoot 
	}

	// if we have successors 
	if len(stmt.successors) >0 {
		// add the head cfgnode for each successor statement 
		for _, succStmt = range stmt.successors {

			if (succStmt.forRoot != nil) {
				succ = succStmt.forRoot 
			} else {
				succ = succStmt
			}

			// if the cfg node is an eos and the statement is a for statement, add the
			// edge to the for conditional cfgNode
			if (len(succ.cfgNodes) > 0) {

				// hack for an eos node to a for statement
				// 
				if ((succ.stmtType == "forStmt") && ( cnode.cfgType == "eos"))  {
					var cond,init,post *CfgNode 
					
					for _, cfgNode := range succ.cfgNodes {
						if (cfgNode.cfgType == "forInit") {
							init = cfgNode 
						}

						if (cfgNode.cfgType == "forCond") {
							cond = cfgNode 
						}
						
						if (cfgNode.cfgType == "forPost") {
							post = cfgNode 
						}						
					}

					// if the eos occurs before the succesor, use the
					// init as the successor, else use the condition
					// assumes there is always a condition, even if implied 
					if (cnode.id < succ.id) {
						if (init != nil) {
							cnode.successors = append(cnode.successors,init)
						} else {
							cnode.successors = append(cnode.successors,cond)
						}
					} else { // eos is after the other node 
						if (post != nil) { // no post statemetn 
							cnode.successors = append(cnode.successors,post)
						} else {
							cnode.successors = append(cnode.successors,cond)
						}
					}

				} else { 
					cnode.successors = append(cnode.successors,succ.cfgNodes[0])
				}
			} else {
				// fmt.Printf("Error at %s stmt node %d no cfg successor \n",_file_line_(),succ.id)
			}
		}
	}
	if (stmt.stmtType != "startNode") { 
		// cnode.predecessors = append(cnode.predecessors, getPredStmtCfg(stmt))
	}

}

// for statements where the child in the next logical statement not the successor statement
func addChildToCfg(cnode *CfgNode, stmt *StatementNode) {
	var succStmt, predStmt, prev *StatementNode

	succStmt = nil
	predStmt = nil

	// if this is a for or if sub-statement, then skip to the for statement
	if (stmt.ifRoot != nil) {
		stmt = stmt.ifRoot 
	}
	
	// if we have successors 
	if stmt.child != nil {
		succStmt = stmt.child
		if (len(succStmt.cfgNodes) > 0) {
			cnode.successors = append(cnode.successors,succStmt.cfgNodes[0])
		}
	}
	// if we have predecessors 
	if len(stmt.predecessors ) > 0 {

		// add the tail cfgNode for each predecessor statement 	
		for _, predStmt = range stmt.predecessors {
			
			if (predStmt.ifRoot != nil) {
				prev = predStmt.ifRoot 
			} else {
				prev = predStmt 
			}

			if (len(prev.cfgNodes) > 0) {
				cnode.predecessors = append(cnode.predecessors,prev.cfgNodes[len(prev.cfgNodes)-1])
			}
		}
	}	
}


// Given a statement Node, return the predecessor control flow graph node
// Must have a function for this as various expressions in for an if statements must get bumped up to
// the parent statement
func getPredStmtCfg(stmt *StatementNode) *CfgNode {

	var predStmt, prev *StatementNode
	// add the tail cfgNode for each predecessor statement 	
	for _, predStmt = range stmt.predecessors {
			
		if (predStmt.ifRoot != nil) {
			prev = predStmt.ifRoot 
		} else {
			if (predStmt.forRoot != nil) {
				prev = predStmt.forRoot 				
			} else {
				prev = predStmt 
			}
		}

		// break and continue need to break the predecessor edges 
		if (prev.stmtType == "breakStmt") || (prev.stmtType == "continueStmt") {
			return nil 
		}
		
		if (len(prev.cfgNodes) > 0) {
			if (prev.forCond != nil) { // is the previous node has a for conditional, make the cond the target
				// not the last node in the list 
				for _, cfgTarget := range prev.cfgNodes {
					if cfgTarget.cfgType == "forCond" { 
						return cfgTarget
					}
				}
			} else { 
				return prev.cfgNodes[len(prev.cfgNodes)-1]
			}
		} else {
			// fmt.Printf("Error at %s stmt node %d no cfg predecessor \n",_file_line_(),prev.id)
		}
	}
	fmt.Printf("Error at %s stmt node %d no cfg node \n",_file_line_(),stmt.id)
	return nil
}

// get the loop head by walking up the parent until we find a for statement 
func getLoopHead(stmt *StatementNode) *StatementNode {
	var foundLoop bool
	var parent,initStmt *StatementNode

	initStmt = stmt 
	parent = stmt
	
	if (parent.stmtType == "forStmt") {
		foundLoop = true 
	}
	
	// walk up the parents looking for the loop head 
	for (foundLoop == false) && (parent != nil) {
		if (parent.stmtType == "forStmt") {
			foundLoop = true 
		} else {
			parent = parent.parent 
		}
	}
	
	// sanity check 
	if (parent == nil) {
		fmt.Printf("Error at %s stmt node %d no loop parent for stmt node %d \n",_file_line_(),initStmt.id)
	}
	
	return parent 
}

	
// build the control flow graph and data flow from the statement graph
/* 
  assignment
  breakStmt
  continueStmt
  eos
  expression
  expressionStmt
  forStmt
  FuncExit
  functionDecl
  goStmt
  ifStmt
  incDecStmt
  returnStmt
  sendStmt
  shortVarDecl
  unaryExpr
*/


// create the list of control flow nodes
// The approach is to create control flow graph nodes 1-to-1 with statement nodes
// and then add/remove edges. Not all of the control flow nodes will end up
// reachable, but we can rely on the 1-1 statement to cfg node propety
// for the forward pass. 
func (l *argoListener) forwardCfgPass() {
	var currentStmt *StatementNode 
	var currentCfgNode *CfgNode 
	var i int 

	// create all the control-flow graph nodes
	// add edges later after all nodes are created 
	for i, currentStmt = range(l.statementGraph) {

		if (currentStmt.visited == true) {
			continue ;
		}
		
		currentStmt.visited = true 

		if (currentStmt.ifRoot != nil ) || (currentStmt.forRoot != nil ) {
			// need assertion to check if the root is pointing to a simple or test statement only 
			continue;
		}
		
		switch currentStmt.stmtType {
		case "assignment":
			_, currentCfgNode = l.newCFGnode(currentStmt, 0)
			currentCfgNode.cfgType = "assignment"
			l.controlFlowGraph = append(l.controlFlowGraph,currentCfgNode)
		case "breakStmt":
			_, currentCfgNode = l.newCFGnode(currentStmt, 0)
			currentCfgNode.cfgType = "break"			
			l.controlFlowGraph = append(l.controlFlowGraph,currentCfgNode)
		case "continueStmt":
			_, currentCfgNode = l.newCFGnode(currentStmt, 0)
			currentCfgNode.cfgType = "continue"						
			l.controlFlowGraph = append(l.controlFlowGraph,currentCfgNode)			
		case "eos":
			_, currentCfgNode = l.newCFGnode(currentStmt, 0)
			currentCfgNode.cfgType = "eos"						
			l.controlFlowGraph = append(l.controlFlowGraph,currentCfgNode)
		case "expression":
			_, currentCfgNode = l.newCFGnode(currentStmt, 0)
			currentCfgNode.cfgType = "expression"						
			l.controlFlowGraph = append(l.controlFlowGraph,currentCfgNode)
		case "expressionStmt":
			_, currentCfgNode = l.newCFGnode(currentStmt, 0)
			currentCfgNode.cfgType = "expression"						
			l.controlFlowGraph = append(l.controlFlowGraph,currentCfgNode)

			// create control flow graph nodes for a for statement
			// if the statement is missing conditional and post nodes
			// we go ahead and add ghost node version of them 
		case "forStmt":

			if (currentStmt.forInit != nil) { 
				_, forCfgInit := l.newCFGnode(currentStmt,0)
				forCfgInit.cfgType = "forInit"
				forCfgInit.subStmt = currentStmt.forInit
				forCfgInit.subStmtID = currentStmt.forInit.id
				l.controlFlowGraph = append(l.controlFlowGraph,forCfgInit)	
				currentStmt.forInit.visited = true 
			}
			if (currentStmt.forCond != nil ) {
				_, forCfgCond := l.newCFGnode(currentStmt, 1)
				forCfgCond.cfgType = "forCond"
				forCfgCond.subStmt = currentStmt.forCond
				forCfgCond.subStmtID = currentStmt.forCond.id 
				l.controlFlowGraph = append(l.controlFlowGraph,forCfgCond)
				currentStmt.forCond.visited = true 
			} else {
				_, forCfgCond := l.newCFGnode(currentStmt, 4)
				forCfgCond.cfgType = "forCond"
				forCfgCond.subStmt = nil
				forCfgCond.subStmtID = -1
				l.controlFlowGraph = append(l.controlFlowGraph,forCfgCond)
			}

			if (currentStmt.forPost != nil ) {
				_, forCfgPost := l.newCFGnode(currentStmt, 2)
				forCfgPost.cfgType = "forPost"
				forCfgPost.subStmt = currentStmt.forPost
				forCfgPost.subStmtID = currentStmt.forPost.id
				l.controlFlowGraph = append(l.controlFlowGraph,forCfgPost)	
				currentStmt.forPost.visited = true 
			} else {
				_, forCfgPost := l.newCFGnode(currentStmt, 5)
				forCfgPost.cfgType = "forPost"
				forCfgPost.subStmt = nil 
				forCfgPost.subStmtID = -1
				l.controlFlowGraph = append(l.controlFlowGraph,forCfgPost)
			}
			
		case "FuncExit":
			_, currentCfgNode = l.newCFGnode(currentStmt, 0)
			currentCfgNode.cfgType = "funcExit"
			l.controlFlowGraph = append(l.controlFlowGraph,currentCfgNode)
		case "functionDecl":
			_, currentCfgNode = l.newCFGnode(currentStmt, 0)
			currentCfgNode.cfgType = "funcEntry"
			l.controlFlowGraph = append(l.controlFlowGraph,currentCfgNode)
		case "goStmt":
			_, currentCfgNode = l.newCFGnode(currentStmt, 0)
			currentCfgNode.cfgType = "goStmt"
			l.controlFlowGraph = append(l.controlFlowGraph,currentCfgNode)			
		case "ifStmt":

			if (currentStmt.ifSimple != nil) { 
				_, ifSimpleCfg := l.newCFGnode(currentStmt,0)
				ifSimpleCfg.cfgType = "ifSimple"
				ifSimpleCfg.subStmt = currentStmt.ifSimple
				ifSimpleCfg.subStmtID = currentStmt.ifSimple.id 
				l.controlFlowGraph = append(l.controlFlowGraph,ifSimpleCfg)
				currentStmt.ifSimple.visited = true 
			}
			
			if (currentStmt.ifTest != nil ) {
				_, ifTestCfg := l.newCFGnode(currentStmt, 1)
				ifTestCfg.cfgType = "ifTest"
				ifTestCfg.subStmt = currentStmt.ifTest
				ifTestCfg.subStmtID = currentStmt.ifTest.id 
				l.controlFlowGraph = append(l.controlFlowGraph,ifTestCfg)
				currentStmt.ifTest.visited = true 

			}
			
		case "incDecStmt":
			_, currentCfgNode = l.newCFGnode(currentStmt, 0)
			currentCfgNode.cfgType = "incDec"
			l.controlFlowGraph = append(l.controlFlowGraph,currentCfgNode)
		case "returnStmt":
			_, currentCfgNode = l.newCFGnode(currentStmt, 0)
			currentCfgNode.cfgType = "return"
			l.controlFlowGraph = append(l.controlFlowGraph,currentCfgNode)
		case "sendStmt":
			_, currentCfgNode = l.newCFGnode(currentStmt, 0)
			currentCfgNode.cfgType = "send"
			l.controlFlowGraph = append(l.controlFlowGraph,currentCfgNode)			
		case "shortVarDecl":
			_, currentCfgNode = l.newCFGnode(currentStmt, 0)
			currentCfgNode.cfgType = "shortVarDecl"
			l.controlFlowGraph = append(l.controlFlowGraph,currentCfgNode)						
		case "unaryExpr":
			_, currentCfgNode = l.newCFGnode(currentStmt, 0)
			currentCfgNode.cfgType = "unaryExpr"
			l.controlFlowGraph = append(l.controlFlowGraph,currentCfgNode)
		case "startNode":
			_, currentCfgNode = l.newCFGnode(currentStmt, 0)
			currentCfgNode.cfgType = "startNode"
			l.controlFlowGraph = append(l.controlFlowGraph,currentCfgNode)
		case "finishNode":
			_, currentCfgNode = l.newCFGnode(currentStmt, 1)
			currentCfgNode.cfgType = "finishNode"
			l.controlFlowGraph = append(l.controlFlowGraph,currentCfgNode)
		default:
			fmt.Printf("Error at %s node %d unknown statement type: %s \n",_file_line_(),i,currentStmt.stmtType)
		}

		
	}


	for _, currentStmt = range(l.statementGraph) {
		currentStmt.visited = false 
	}
	// proceeded linearly down the sequence of statements
	// and create edges in the control flow graph 
	for _, currentStmt = range(l.statementGraph) {
		if (currentStmt.visited == true) {
			continue 
		}
		
		currentStmt.visited = true
		if len(currentStmt.cfgNodes) == 0 {
			continue 
		} else { 
			currentCfgNode = currentStmt.cfgNodes[0]
		}

		switch currentStmt.stmtType {

		case "startNode":
			addLinearToCfg(currentCfgNode,currentStmt)
		case "assignment":
			addLinearToCfg(currentCfgNode,currentStmt)
			// get the write variable and add it to the list of variables 
			for _, varNode := range( currentStmt.writeVars) {
				varNode.cfgNodes = append(varNode.cfgNodes,currentCfgNode) 
			}
		case "breakStmt": // walk up to the first loop 
			var loopHead *StatementNode

			loopHead = getLoopHead(currentStmt)
			targetSuccessor := loopHead.successors[0].cfgNodes[0]
			currentCfgNode.successors = append(currentCfgNode.successors,targetSuccessor)
			
			predCfg := getPredStmtCfg(currentStmt)
			currentCfgNode.predecessors = append(currentCfgNode.predecessors,predCfg)
			
		case "continueStmt":
			var loopHead *StatementNode
			var condCfg  *CfgNode
			
			loopHead = getLoopHead(currentStmt)
			// find the conditional node 
			for _, condCfg = range loopHead.cfgNodes {
				if condCfg.cfgType == "forCond" {
					break
				}
			}

			currentCfgNode.successors = append(currentCfgNode.successors,condCfg)

			
			predCfg := getPredStmtCfg(currentStmt)
			currentCfgNode.predecessors = append(currentCfgNode.predecessors,predCfg)
			
		case "eos":
			addLinearToCfg(currentCfgNode,currentStmt)
		case "expression":
			addLinearToCfg(currentCfgNode,currentStmt)
		case "expressionStmt":
			addLinearToCfg(currentCfgNode,currentStmt)
		case "finishNode":
			addLinearToCfg(currentCfgNode,currentStmt)
		case "forStmt":
			var initCfg,condCfg,postCfg,blockCfg,prevCfg,eosCfg *CfgNode
			var endStmt *StatementNode

			// last (tail) control node for the prev statement 
			//prevCfg = prevStmt.cfgNodes[len(prevStmt.cfgNodes)-1]
			prevCfg = getPredStmtCfg(currentStmt)
			
			// the head of the block for the loop 
			if (currentStmt.forBlock != nil) { 
				blockCfg = currentStmt.forBlock.cfgNodes[0]
			}

			// the EOS for the loop; this is the loop exit 
			endStmt = currentStmt.successors[0]
			eosCfg = endStmt.cfgNodes[0]

			for _,controlNode := range (currentStmt.cfgNodes) {
				
				switch controlNode.cfgType {
				case "forInit":
					initCfg = controlNode
				case "forCond":
					condCfg = controlNode 
				case "forPost":
					postCfg = controlNode 
				default:
					fmt.Printf("Error: at %s forStmt %d has unknown type %s \n",_file_line_(),controlNode.id,controlNode.cfgType)					
				}
			}

			// if there is an initialization node, connect the conditional node
			// if there is one. 
			if (initCfg != nil){
				// connect to the tail of the previous node 
				initCfg.predecessors = append(initCfg.predecessors,prevCfg)

				// if there is a config node 
				if (condCfg != nil) {
					initCfg.successors = append(initCfg.successors,condCfg)
					condCfg.predecessors = append(condCfg.predecessors,initCfg)
				} else {
					initCfg.successors = append(initCfg.successors,blockCfg)
				}

				// we need to add the list of variales to the init statement 
				stmtNode := initCfg.subStmt
				// add the init condition variable node 
				for _, varNode := range(stmtNode.writeVars) {
					varNode.cfgNodes = append(varNode.cfgNodes,initCfg) 
				}
				
			}

			// main clause if there is a config node 
			if (condCfg != nil) {
				condCfg.successors_taken = append(condCfg.successors_taken,blockCfg)
				condCfg.successors = append(condCfg.successors,eosCfg)
			} else {
				if (initCfg == nil) {
					condCfg.predecessors = append(condCfg.successors,prevCfg)
				}
			}
			

			// main clause if there is a post config statement 
			// if there is a post (end of loop) statement

			if (postCfg != nil) {
				tailStmt := currentStmt.forTail
				tailCfg := tailStmt.cfgNodes[0]

				
				if (condCfg != nil) {
					postCfg.successors = append(postCfg.successors,condCfg)
					condCfg.predecessors = append(condCfg.predecessors,postCfg)
				} else {
					eosCfg.successors= append(eosCfg.successors,prevCfg)
				}
				postCfg.predecessors = append(postCfg.predecessors,tailCfg)


				/// the post statement might have write variables
				if postCfg.subStmt != nil { 
					stmtNode := postCfg.subStmt
					// add the post condition variable node 
					for _, varNode := range(stmtNode.writeVars) {
						varNode.cfgNodes = append(varNode.cfgNodes,postCfg) 
					}
				}
				
			} else {
				if (condCfg != nil) {
					eosCfg.successors= append(eosCfg.successors,condCfg)
				} else {
					eosCfg.successors= append(eosCfg.successors,blockCfg)
				}
			}
				
		case "FuncExit":
			addLinearToCfg(currentCfgNode,currentStmt)
		case "functionDecl":
			addChildToCfg(currentCfgNode,currentStmt)
		case "goStmt":
			addLinearToCfg(currentCfgNode,currentStmt)
		case "ifStmt":
			var simpleCfg,testCfg, takenCfg, elseCfg *CfgNode

			// add the simple statement to the front of the test 
			if (currentStmt.ifSimple != nil) {
				simpleCfg =  currentStmt.cfgNodes[0]
				testCfg =   currentStmt.cfgNodes[1]
				simpleCfg.successors = 	append(simpleCfg.successors, testCfg)
				testCfg.predecessors = 	append(testCfg.predecessors,simpleCfg) 
			} else {
				testCfg =   currentStmt.cfgNodes[0]				
			}

			if (len(currentStmt.ifTaken.cfgNodes) > 0) {
				takenCfg = currentStmt.ifTaken.cfgNodes[0]
			} else {
				fmt.Printf("Error: at %s ifstmt taken %d has no cfg node \n",_file_line_(),currentStmt.id)
			}

			
			//create the links to the test node 
			testCfg.successors_taken = append(testCfg.successors_taken,takenCfg)
			if (currentStmt.ifElse != nil) {
				if (len(currentStmt.ifElse.cfgNodes) > 0) {
					elseCfg = currentStmt.ifElse.cfgNodes[0]
				} else {
					fmt.Printf("Error at %s ifstmt else %d else haa no cfg node \n",_file_line_(),currentStmt.id)
				}
				testCfg.successors = append(takenCfg.successors,elseCfg)
			} else {
				succStmt := currentStmt.successors[0]
				testCfg.successors = append(takenCfg.successors,succStmt.cfgNodes[0])
			}
			
		case "incDecStmt":
			addLinearToCfg(currentCfgNode,currentStmt)
		case "returnStmt":
			addLinearToCfg(currentCfgNode,currentStmt)
		case "sendStmt":
			addLinearToCfg(currentCfgNode,currentStmt)
		case "shortVarDecl":
			addLinearToCfg(currentCfgNode,currentStmt)
		case "unaryExpr":
			addLinearToCfg(currentCfgNode,currentStmt)
		default:
			fmt.Printf("Error at %s unknown statement type: %s \n",_file_line_(),currentStmt.id)			
		}

	}

	// change the iftest targets predecessors to the predecessors taken 
	for _, currentCfgNode = range(l.controlFlowGraph) {
		if (currentCfgNode.cfgType == "ifTest") || (currentCfgNode.cfgType == "forCond") {
			for _, cNode := range currentCfgNode.successors_taken {
				cNode.predecessors_taken = append(cNode.predecessors_taken,currentCfgNode)
				cNode.predecessors = removeCfgFromList(cNode.predecessors, currentCfgNode)
			}
			
		}

	}

	
}

// fix the backward edges and make sure the graph is consistent
// every forward edge must have a backward edge
// assumes all the forward edges are correct 
func (l *argoListener) fixBackwardCfgEdges() {
	var foundit bool
	
	// for each node in the cfg graph 
	for _, cNode := range(l.controlFlowGraph) {
		// for each successor of the cfgnode
		for _,succ := range(cNode.successors) {
			foundit = false 
			for _, pred := range(succ.predecessors) {
			// look for the parent node in the predecessor list
				if (pred.id == cNode.id ) {
					foundit = true 
					break; 
				}

			}
			// didn't find the parent in the list of successors 
			if foundit == false {
				succ.predecessors = append(succ.predecessors,cNode)
			}
		} // for each successor


		// now do the sucessors-taken nodes
		for _,succ := range(cNode.successors_taken) {
			foundit = false 
			for _, pred := range(succ.predecessors_taken) {
				// look for the parent node in the predecessor list
				if (pred.id == cNode.id ) {
					foundit = true 
					break; 
				}

			}
			// didn't find the parent in the list of successors 
			if foundit == false {
				succ.predecessors_taken = append(succ.predecessors_taken,cNode)
			}
		} // for each successors_taken 
		
	} // for each node in the control-flow graph 
}

// add the list of variables that write in a CFG node to the CFG graph
func (l *argoListener) addVarsToCfgNodes() {
	for _, vNode := range(l.varNodeList) {
		for _, cNode := range vNode.cfgNodes {
			cNode.writeVars = append(cNode.writeVars,vNode)
		}
	}
}
// for now, insert an empty control flow node after every write node
// need to fix this to property look for the read/write vars and only
// add a bubble if there is a read after a write of the same variable 

func (l *argoListener) resolveDataflowHazards() {
	var stmtNode  *StatementNode
	var bubbleCfgNode *CfgNode
	var cfgPosition int
	
	for _, cNode := range(l.controlFlowGraph) {

	
		// if there are writevars, add a new control node which does nothing
		// the links will change as below:
		// predecessors p_taken         Pred   p_takn
		// |               |             V      V
		// V               V            orig Node
		//   -----------                     | successor edge 
		//  | Orig node|                     V  
		//   ----------                |---bubble--_|
		// V              V            V            V
		// sucessors     s_taken      sucessors s_taken 
		if len(cNode.writeVars) > 0 {  // fixme: change to check for read after write 
			// create a new CFG node
			stmtNode = cNode.statement

			// in order to give the node a unique name, we have to find the positions
			// of the cfg node in the statement node and add 7 to it
			for i, cfgPos := range stmtNode.cfgNodes {
				if cfgPos.id == cNode.id {
					cfgPosition = i;
					break; 
				}
			}
			_, bubbleCfgNode = l.newCFGnode(stmtNode, cfgPosition + 9)
			
			bubbleCfgNode.cfgType = "bubble"
			l.controlFlowGraph = append(l.controlFlowGraph,bubbleCfgNode)

			// copy the successors edges to the bubble node 
			bubbleCfgNode.successors_taken = append(bubbleCfgNode.successors_taken,cNode.successors_taken...)
			bubbleCfgNode.successors = append(bubbleCfgNode.successors,cNode.successors...)

			// set the successors_taken and successors of the cNode predecessors to the bubble
			// for each successor 
			for _, succNode := range cNode.successors {
				// for each predecessors in the successsor 
				for j, predNode := range succNode.predecessors {
					// swap the orignal cnode with the bubble 
					if predNode.id == cNode.id {
						succNode.predecessors[j] = bubbleCfgNode
					}
				}
			}

			// do the same thing for the successors_taken node 
			for _, succNode := range cNode.successors_taken {
				// for each predecessors in the successsor 
				for j, predNode := range succNode.predecessors_taken {
					// swap the orignal cnode with the bubble 
					if predNode.id == cNode.id {
						succNode.predecessors_taken[j] = bubbleCfgNode
					}
				}
			}

			
			// set the orig node successor to the bubble
			cNode.successors = nil
			cNode.successors_taken = nil
			cNode.successors = append(cNode.successors, bubbleCfgNode)

			// set the bubble predecessor to the orig node 
			bubbleCfgNode.predecessors =  append(bubbleCfgNode.predecessors, cNode)

		}
	}
}


// top level function to get the control flow graph
func (l *argoListener) getControlFlowGraph() int {

	// call the forward pass on the control-flow graph 
	l.forwardCfgPass()
	// fix backward edges
	l.fixBackwardCfgEdges() 
	// link the variable write/reads to the control flow graph nodes 
	l.addVarsToCfgNodes()
	// add delays in the cfg when there are data flow hazards
	l.resolveDataflowHazards()
	return 1 
}

/* ******************  Print Structures Section   ************************* */

func (l *argoListener) printControlFlowGraph() {
	// sort by id number 
	sort.Slice(l.controlFlowGraph, func(i, j int) bool {
		return l.controlFlowGraph[i].id < l.controlFlowGraph[j].id
	})
		
	for i, node := range l.controlFlowGraph {
		fmt.Printf("Cntl: %d: ID:%d stmt:%d :%s: %s succ: ", i,node.id,node.statement.id,node.cannName,node.cfgType)

		for _,s:= range node.successors { 
			fmt.Printf("%d ",s.id)
		}

		fmt.Printf(" s_taken: ")

		for _,st := range node.successors_taken { 
			fmt.Printf("%d ",st.id)
		}

		fmt.Printf(" pred: ")
		
		for _,p := range node.predecessors {
			if p == nil {
				fmt.Printf("-")				
			} else { 
				fmt.Printf("%d ",p.id)
			}
		}

		fmt.Printf(" p_taken: ")
		for _,pt := range node.predecessors_taken { 
			fmt.Printf("%d ",pt.id)
		}
		
		
		fmt.Printf("\n")		
	}

}

func printStatementList(stmts []*StatementNode) {
	if (len(stmts) == 0) {
		fmt.Printf("\t\t <none> \n ")
	} else { 
		for i, stmt := range stmts {
			fmt.Printf("\t\t stmt index %d node id %d \n ",i,stmt.id)
		}
	}

}

// print all the ParseTree nodes. Can be in rawWithText mode, which includes the source code with each node, or
// in dotShort mode, which is a graphViz format suitable for making graphs with the dot program 
func (l *argoListener) printParseTreeNodes(outputStyle string) {

	var nodeStr string // name of the AST node 
	
	if (outputStyle == "rawWithText") { 
		for _, node := range l.ParseNodeList {
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
		for _, node := range l.ParseNodeList {
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
		for _, node := range l.ParseNodeList {
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
		fmt.Printf("Variable: %d name: %s func: %s pos:(%d,%d) class:%s prim:%s size:%d param:%t result:%t ",
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

// print the list of functions 
func (l *argoListener) printFuncNodes() {
	for _, node := range l.funcNodeList {
		fmt.Printf("Func: %d: name %s at (%d,%d) ", node.id,node.funcName,node.sourceRow,node.sourceCol)
		if (len(node.parameters) >0) {
			fmt.Printf("params: ")
			for _, param := range(node.parameters) {
				fmt.Printf("[%s:%s:%d] ",param.sourceName,param.goLangType,param.numBits)
			}
		}
		if ( len(node.retVars) >0 ) {
			fmt.Printf("retVars: ")
			for _, param := range(node.retVars) {
				fmt.Printf("[%s:%s:%d] ",param.sourceName,param.goLangType,param.numBits)
			}			
		}
		fmt.Printf("\n")
	}
}

func (l *argoListener) printVarScopes() {
	var scope *VarScope
	// sort statements by id number 
	sort.Slice(l.statementGraph, func(i, j int) bool {
		return l.statementGraph[i].id < l.statementGraph[j].id
	})


	for i, node := range l.statementGraph {
		if (node.vScope != nil) {
			fmt.Printf("Stmt: %d ID:%d scopeId:%d at (%d,%d) type:%s parent: %d child: %d scopeVars: ", i , node.id, node.vScope.id, node.sourceRow, node.sourceCol, node.stmtType,node.parentID,node.childID)

			scope = node.vScope
			for vName := range scope.varNameMap {
				varNode := scope.varNameMap[vName]
				fmt.Printf("%s:%s ",vName,varNode.canName)
			}
			fmt.Printf("\n")			
		} else { 
			fmt.Printf("Stmt: %d ID:%d scopeId:nil at (%d,%d) type:%s parent: %d child: %d scopeVars: ", i , node.id, node.sourceRow, node.sourceCol, node.stmtType,node.parentID,node.childID)
		}


	}
}

// print the statement graph
func (l *argoListener) printStatementGraph(format string) {
	var j int

	// sort by id number 
	sort.Slice(l.statementGraph, func(i, j int) bool {
		return l.statementGraph[i].id < l.statementGraph[j].id
	})

	if (format == "graphViz") {
		fmt.Printf("Digraph G { \n")
	}
	
	for i, node := range l.statementGraph {

		if (format == "text") {
			fmt.Printf("Stmt: %d: ID:%d at (%d,%d) type:%s parent: %d child: %d pred: ", i,node.id, node.sourceRow, node.sourceCol, node.stmtType,node.parentID,node.childID)
		}

		if (format == "graphViz") {
			parent := node.parent
			child := node.child
			if (parent != nil) {
				fmt.Printf("\"%d%s\" -> \"%d%s\" [ label = \"pa\" ]; \n",node.id,node.stmtType,parent.id,parent.stmtType)
			}
			if (child != nil) {
				fmt.Printf("\"%d%s\" -> \"%d%s\" [ label = \"ch\" ]; \n",node.id,node.stmtType,child.id,child.stmtType)
			}
		}
		
		// assertion checks:
		if (len(node.predecessors) != len(node.predIDs)) {
			fmt.Printf("Error: length of precedessors does not match %d %d \n",len(node.predecessors),len(node.predIDs))
		}
		if (len(node.successors) != len(node.succIDs)) {
			fmt.Printf("Error: length of successors does not match %d %d \n",len(node.successors),len(node.succIDs))
		}

		for i,id := range node.predIDs { 
			if (format == "text") {
				fmt.Printf("%d ",id)
			}
			if (format == "graphViz") {
				pred := node.predecessors[i]
				fmt.Printf("\"%d%s\" -> \"%d%s\" [ label = \"pr\" ]; \n",node.id,node.stmtType,pred.id,pred.stmtType)
			}
		}

		if (format == "text") {		
			fmt.Printf(" succ: ")
		}
		j = 0 
		for (j < len(node.succIDs)) {
			if (format == "text") {
				fmt.Printf("%d ",node.succIDs[j])

			}
			if (format == "graphViz") {
				succ := node.successors[j]
				fmt.Printf("\"%d%s\" -> \"%d%s\" [ label = \"su\" ]; \n",node.id,node.stmtType,succ.id,succ.stmtType)
			}
			j++
		}		

		if ((node.ifRoot != nil)  && format == "text") {
			fmt.Printf("ifroot: %d ",node.ifRoot.id)
		}

		if ((node.forRoot != nil) && format == "text") {
			fmt.Printf("forroot: %d ",node.forRoot.id)			
		}
					
		if len(node.callTargets) >0 {
			if (format == "text") {			
				fmt.Printf(" callto: ")
			}
			j=0
			for (j < len(node.callTargets)) {

				if (format == "text") {
					fmt.Printf("%d ",node.callTargets[j].id)
				}
				if (format == "graphViz") {
					callee := node.callTargets[j]
					fmt.Printf("\"%d%s\" -> \"%d%s\" [ label = \"ca\" ]; \n",node.id,node.stmtType,callee.id,callee.stmtType)					
				}
				j++
			}		
		}

		if len(node.callers) >0 {
			if (format == "text") {			
				fmt.Printf(" callers: ")
			}
			j=0
			for (j < len(node.callers)) {

				if (format == "text") {
					fmt.Printf("%d ",node.callers[j].id)
				}
				if (format == "graphViz") {
					caller := node.callers[j]
					fmt.Printf("\"%d%s\" -> \"%d%s\" [ label = \"ct\" ]; \n",node.id,node.stmtType,caller.id,caller.stmtType)					
				}
				j++
			}		
		}

		if len(node.returnTargets) >0 {
			if (format == "text") {			
				fmt.Printf(" return: ")
			}
			j=0
			for (j < len(node.returnTargets)) {
				if (format == "text") {
					fmt.Printf("%d ",node.returnTargets[j].id)
				}
				if (format == "graphViz") {
					ret := node.returnTargets[j]
					fmt.Printf("\"%d%s\" -> \"%d%s\" [ label = \"ca\" ]; \n",node.id,node.stmtType,ret.id,ret.stmtType)										
				}
				j++
			}		
		}

		if len(node.writeVars) >0 {
			if (format == "text") {			
				fmt.Printf(" writeVars: ")
				for _, varNode := range( node.writeVars) {
					fmt.Printf("%s_%d  ", varNode.sourceName,varNode.id)
				}
			}
		}
		// Get sub statement lists for this node
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
			if (format == "text") {						
				fmt.Printf("simple: %d test: %d taken %d else %d ",node.ifSimpleID(),node.ifTestID(),node.ifTakenID(),node.ifElseID())
			}

			if (format == "graphViz") {
				ifSimple := node.ifSimple
				ifTest := node.ifTest
				ifTaken := node.ifTaken
				ifElse := node.ifElse

				if (ifSimple != nil) {
					fmt.Printf("\"%d%s\" -> \"%d%s\" [ label = \"ifs\" ]; \n",node.id,node.stmtType,ifSimple.id,ifSimple.stmtType)
				}
				if (ifTest != nil) {
					fmt.Printf("\"%d%s\" -> \"%d%s\" [ label = \"its\" ]; \n",node.id,node.stmtType,ifTest.id,ifTest.stmtType)
				}
				if (ifTaken != nil) {
					fmt.Printf("\"%d%s\" -> \"%d%s\" [ label = \"ita\" ]; \n",node.id,node.stmtType,ifTaken.id,ifTaken.stmtType)
				}
				
				if (ifElse != nil) {
					fmt.Printf("\"%d%s\" -> \"%d%s\" [ label = \"iel\" ]; \n",node.id,node.stmtType,ifElse.id,ifElse.stmtType)
				}
			}
			
		case "switchStmt":
		case "selectStmt":
		case "forStmt":
			if (format == "text") {						
				fmt.Printf("init: %d cond: %d post %d block %d tail %d ",node.forInitID(),node.forCondID(),node.forPostID(),node.forBlockID(),node.forTailID())
			}


			if (format == "graphViz") {
				forInit := node.forInit
				forCond := node.forCond
				forPost := node.forPost
				forBlock := node.forBlock

				if (forInit != nil) {
					fmt.Printf("\"%d%s\" -> \"%d%s\" [ label = \"fin\" ]; \n",node.id,node.stmtType,forInit.id,forInit.stmtType)
				}
				if (forCond != nil) {
					fmt.Printf("\"%d%s\" -> \"%d%s\" [ label = \"its\" ]; \n",node.id,node.stmtType,forCond.id,forCond.stmtType)
				}
				if (forPost != nil) {
					fmt.Printf("\"%d%s\" -> \"%d%s\" [ label = \"ita\" ]; \n",node.id,node.stmtType,forPost.id,forPost.stmtType)
				}
				
				if (forBlock != nil) {
					fmt.Printf("\"%d%s\" -> \"%d%s\" [ label = \"iel\" ]; \n",node.id,node.stmtType,forBlock.id,forBlock.stmtType)
				}
			}
			
		case "sendStmt":
		case "expressionStmt":
		case "incDecStmt":
		case "assignment":
		case "shortVarDecl":
		case "emptyStmt":

		default:
			Pass()
		}

		fmt.Printf("\n")		
	}
	if (format == "graphViz") {
		fmt.Printf("} \n")
	}
}


/* ******************  Parse Tree Contruction Section   ************************* */

// recursive function to visit nodes in the Antlr4 graph 
func VisitNode(l *argoListener,c antlr.Tree, parent *ParseNode,level int) ParseNode {
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

	thisNode := ParseNode{id : id , ruleType : ruleName, parentID: parent.id, parent: parent, sourceCode: progText , isTerminal : isTerminalNode, sourceLineStart: startline, sourceColStart : startcol, sourceLineEnd : stopline, sourceColEnd : stopcol }
	thisNode.visited = false
	
	for i := 0; i < c.GetChildCount(); i++ {
		child := c.GetChild(i)
		childParseNode := VisitNode(l,child,&thisNode,mylevel)
		thisNode.children = append(thisNode.children,&childParseNode) 
		thisNode.childIDs = append(thisNode.childIDs,childParseNode.id)
	}
	
	l.addParseNode(&thisNode)
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
	root := ParseNode{ id : id, parentID : 0, parent : nil , ruleType: "SourceFile", sourceCode : "WholeProgramText" } 


	
	for i := 0; i < c.GetChildCount(); i++ {
		child := c.GetChild(i)
		// fmt.Printf(" child %d: %p \n",i,child)
		childNode := VisitNode(l,child,&root,level)
		root.children = append(root.children, &childNode)
		root.childIDs = append(root.childIDs, childNode.id)		
		
	}
	// add the root back in
	l.root = &root
	l.addParseNode(&root)

	// sort all the nodes by nodeID in the list of nodes 
	sort.Slice(l.ParseNodeList, func(i, j int) bool {
		return l.ParseNodeList[i].id < l.ParseNodeList[j].id
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
		fmt.Printf("Whoaa! Didn't get any program lines\n")
		
	}
	listener.ProgramLines = progLines

	listener.nextParseID = 0
	listener.ParseNode2ID = make(map[interface{}]int)

	listener.nextVarID = 0

	listener.nextStatementID = 0
	listener.nextCfgID = 0
	
	listener.funcNameMap = make(map[string]*FunctionNode)
	
	listener.logIt.flags = make(map[string]bool,16)
	listener.logIt.init()
	listener.logIt.flags["MIN"] = true

	nName := strings.TrimSuffix(*fname,".go")
	sNames := strings.Split(nName,"/")
	
	listener.moduleName = sNames[len(sNames)-1]
	
	if (err != nil) {
		fmt.Printf("Getting program lines failed\n")
		os.Exit(-1)
	}

	//listener.logIt.DbgLog("MIN","testing the log %d %d %d \n",5,10,20)
	
	// Finally parse the expression (by walking the tree)
	antlr.ParseTreeWalkerDefault.Walk(listener, p.SourceFile())
	
	if (errorCount.syntaxErrors > 0) {
		fmt.Printf("Parsing of program halted due to syntax errors \n");
		os.Exit(1)

	}
	return listener
}

// this is a global variable to abort when there are too
// many errors
var max_parse_errors int

func main() {
	var parsedProgram *argoListener 
	var inputFileName_p,outputFileName_p *string
	var printASTasGraphViz_p,printASTasText_p,printVarNames_p,printFuncNames_p,printStmtGraph_p,parseCheck_p,printScopes_p *bool
	var printStmtGraphGV_p *bool
	var printCntlGraph_p *bool
	var debugFlags   uint64
	var debugFlags_p *string 
	
	inputFileName_p = nil
	outputFileName_p = nil
	max_parse_errors = 50
	debugFlags = 0
	
	printASTasGraphViz_p = flag.Bool("gv",false,"print the parse tree in GraphViz format")
	printASTasText_p = flag.Bool("parse",false,"print the parse tree in text format")	
	printVarNames_p = flag.Bool("vars",false,"print all variables")
	printStmtGraph_p = flag.Bool("stmt",false,"print the statement graph")
	printStmtGraphGV_p = flag.Bool("stmtgv",false,"print the statement graph in graphviz format")
	printFuncNames_p = flag.Bool("func",false,"print all functions")
	printCntlGraph_p = flag.Bool("cntl",false,"print the control-flow graph")
	printScopes_p = flag.Bool("scope",false,"print variable scopes")
	parseCheck_p     = flag.Bool("check",false,"check for correct syntax ")

	debugFlags_p     = flag.String("dbg","","debug flags 1=verilog control ")
	inputFileName_p = flag.String("i","","the input file name")
	outputFileName_p = flag.String("o","","the output file name")


	flag.Parse()

	if (*inputFileName_p == "") {
		fmt.Printf("No input file specified, exiting \n")
		os.Exit(-1)
	} else { 
		parsedProgram = parseArgo(inputFileName_p)
	}

	if ( !( *debugFlags_p == "")) {
		d, err := strconv.ParseInt(*debugFlags_p,10,64)
		if (err != nil) {
			fmt.Printf("error setting debug flags, exiting \n")
			os.Exit(-1)
		} else {
			debugFlags = uint64(d)
		}
	}

	parsedProgram.debugFlags = debugFlags
	
	// these are the top-level main causes of the compiler 
	parsedProgram.getAllVariables()  // must call get all variables first 
	parsedProgram.getAllFunctions()  // then get all functions 
	parsedProgram.getStatementGraph()  // now make the statementgraph

	// adding technical debit 
	// FIXME need to add this back in to fix the scoping rules ... later
	// parsedProgram.fixVariableScopes()  fix the scoping rules to allow for short var decls
	parsedProgram.getControlFlowGraph()  // now make the statementgraph

	
	if (*printASTasGraphViz_p) {
		parsedProgram.printParseTreeNodes("dotShort")
	}

	if (*printASTasText_p) {
		parsedProgram.printParseTreeNodes("rawWithText")
	}
	
	if (*printVarNames_p) {
		parsedProgram.printVarNodes()
		
	}

	if (*printFuncNames_p) {
		parsedProgram.printFuncNodes()
		
	}
	
	
	if (*printStmtGraph_p) {
		parsedProgram.printStatementGraph("text")	
	}
	if (*printStmtGraphGV_p) {
		parsedProgram.printStatementGraph("graphViz")	
	}
	

	if (*printCntlGraph_p)  {
		parsedProgram.printControlFlowGraph()
			
	}

	if (*printScopes_p) {
		parsedProgram.printVarScopes()
		
	}
	
	if (*parseCheck_p) {
		// fmt.Printf("parse check completed \n")
	} 


	if ( len(*outputFileName_p) > 0 ) {
		var w *os.File 
		if *outputFileName_p == "-" {
			w = os.Stdout
		} else {
		
			file, err := os.OpenFile(*outputFileName_p, os.O_WRONLY|os.O_CREATE, 0666)
			if err != nil {
				fmt.Printf("Error opening file %s \n ",outputFileName_p)
				os.Exit(1)
			}
			defer file.Close()
			w = file
		}
		parsedProgram.outputFile = w
		OutputVerilog(parsedProgram);
	}
}
