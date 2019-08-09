
/* Argo to Verilog Compiler 

    (c) Richard P. Martin and contributers 
    
    This program is free software: you can redistribute it and/or modify
    it under the terms of the GNU General Public License as published by
    the Free Software Foundation, either version 3 of the License, or
    (at your option) any later version.

    This program is distributed WITHOUT ANY WARRANTY; without even the implied warranty of
    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
    GNU General Public License version 3 for more details.

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
  C   Map section --- create all the associate arrays. Each map is a CAM
  D   Variable control --- always block to control writes to each variable
  E   Control flow --- bit-vectors for control flow for each function 

*/

package main

import (
	"fmt"
	"os"
	"flag"
	"strings"
	"strconv"
	"bufio"
	"errors"
	"sort"
	// "bytes"
	"./parser"
	"github.com/antlr/antlr4/runtime/Go/antlr"
)


// This is the representation of the AST we use 
type astNode struct {
	id int 
	ruleType string
	isTerminal bool
	parentID int
	parent *astNode
	childIDs []int
	children []*astNode
	sourceCode string
}

type variableNode struct {
	id int                // every var gets an unique ID 
	sourceName string     // name in the source code
	canName string        // cannonical name for Verilog 
	varType string        // type of this variable
	numDim   int          // number of dimension if an array
	funcName  string      // where is the variable defined
	scope  string         // what the scope in terms 
}


type argoListener struct {
	*parser.BaseArgoListener
	stack []int
	recog antlr.Parser
	ProgramLines []string // the program as a list of strings, one string per line
	node2ID map[interface{}]int //  a map of the AST node pointers to small integer ID mapping
	nextID int
	root *astNode                 // root of an absract syntax tree 
	astNodeList []*astNode        // list of nodes of an absract syntax tree, root has id = 0 
	varNodeList []*variableNode   // list of all variables in the program
	varNodeNameMap map[string]*variableNode  // map of cannonical names to variable nodes 
}

// get a node ID in the AST tree 
func (l *argoListener) getID(c antlr.Tree) int {

	// if the entry is in the table, return the integer ID
	if val, ok := l.node2ID[c] ; ok {
		return val 
	}

	// create a new entry and return it 
	l.node2ID[c] = l.nextID
	l.nextID = l.nextID + 1
	return 	l.node2ID[c]
}

// add a node to the list of all nodes
func (l *argoListener) addASTnode(n *astNode) {
	// need to check for duplicates
	l.astNodeList = append(l.astNodeList,n) 
}

func (l *argoListener) addVarnode(v *variableNode) {
	l.varNodeList = append(l.varNodeList,v) 
}

// get the function node of a given AST node
// assumes we have to back-up towards the root node 
func (l *argoListener) getFunctionDecl(n *astNode) *astNode {
	var foundit bool
	var parent *astNode
	
	foundit = false
	parent = n.parent
	for (parent != l.root ) && (foundit == false) { 
		if (parent.ruleType == "functionDecl") {
			return parent 
		}
	}

	fmt.Printf("functionDecl for node %d:%s not found \n", n.id, n.ruleType)
	return nil
}
	
// get all the variables in an AST
// We go linearly through all the nodes looking for declaration types
// if we find one, we crawl the children to get the variable's name and type
// we can also crawl backward to get the scope 
func (l *argoListener) getAllVariables() int {
	var funcDecl,funcName *astNode  // AST node of the function and function name 
	// the three type of declarations are: varDecl (var keyword), parameterDecls (in a function signature), and shortVarDecls (:=)
	funcDecl = nil

	// for every AST node, see if it is a declaration
	// if so, name the variable the _function_name_name
	// for multiple instances of go functions, add the instance number 
	for _, node := range l.astNodeList {
		
		// find the enclosing function name 
		if (node.ruleType == "varDecl") || (node.ruleType == "parameterDecl") || (node.ruleType == "shorVarDecl") {
			funcDecl = l.getFunctionDecl(node)
			funcName = nil
			if len(funcDecl.children) > 1 {
				funcName = funcDecl.children[1]
			} else {
				fmt.Printf("Can't find enclosing child node for funcDecl %d:%s \n", funcDecl.id,funcDecl.sourceCode)
			}
			
		}
		// now get the name and type of the actual declaration.
		// getting both the name and type depends on the kind of declaration it is 
		if (node.ruleType == "varDecl") {
			// do simple var declaration
			// create the cannonical name using the function name and local name 
			// get the function name 
			// go down through the children until we get to the identifier with the local names
		} else if (node.ruleType == "parameterDecl") {
			// a parameter declaration 
		} else if (node.ruleType == "shorVarDecl") {
			// 
		}


	}
	return 0 
}
	
// print all the nodes. Can be in rawWithText mode, which includes the source code with each node, or
// in dotShort mode, which is a graphViz format suitable for making graphs with the dot program 
func (l *argoListener) printASTnodes(outputStyle string) {

	var nodeStr string // name of the AST node 
	
	if (outputStyle == "rawWithText") { 
		for _, node := range l.astNodeList {
			fmt.Printf("AST Nodes: %d: %s ::%s:: parent: %d children: ", node.id, node.ruleType, node.sourceCode, node.parentID )
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

// recursive function to visit nodes in the Antlr4 graph 
func VisitNode(l *argoListener,c antlr.Tree, parent *astNode,level int) astNode {
	var progText string
	var err error
	var id int 
	var isTerminalNode bool
	
	mylevel := level + 1
	id = l.getID(c)
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
		startline := start.GetLine()
		startcol := start.GetColumn()
		stopline := stop.GetLine()
		stopcol := stop.GetColumn()
		progText,err = rowscols2String(l.ProgramLines,startline,startcol,stopline,stopcol)
		if (err != nil) {
			//fmt.Printf("RowCols error on program text %s %d:%d to %d:%d ",err,startline,startcol,stopline,stopcol)
			progText = "ERR"
		}
	}
	
	ruleName := antlr.TreesGetNodeText(c,nil,l.recog)

	thisNode := astNode{id : id , ruleType : ruleName, parentID: parent.id, parent: parent, sourceCode: progText , isTerminal : isTerminalNode}

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
	id = l.getID(c)

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
	// Not that rows in the Go array start a zero
	// but in the compiler text number start at 1
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
	var listener argoListener

	input, err := antlr.NewFileStream(*fname)
	
	lexer := parser.NewArgoLexer(input)
	stream := antlr.NewCommonTokenStream(lexer,0)
	
	p := parser.NewArgoParser(stream)

	listener.recog = p
	progLines, err2 := getFileLines(*fname)
	if (err2 != nil) {
		fmt.Printf("Whoaa! didn't program lines")
		
	}
	listener.ProgramLines = progLines

	listener.node2ID = make(map[interface{}]int)
	listener.nextID = 0
	if (err != nil) {
		fmt.Printf("Getting program lines failed\n")
		os.Exit(-1)
	}

	// Finally parse the expression (by walking the tree)
	antlr.ParseTreeWalkerDefault.Walk(&listener, p.SourceFile())
	

	return &listener 
}

func main() {
	var parsedProgram *argoListener 
	var inputFileName_p *string
	var printASTasGraphViz_p *bool
	
	printASTasGraphViz_p = flag.Bool("gv",false,"print AST in GraphViz format")

	inputFileName_p = flag.String("i","input.go","input file name")

	flag.Parse()
	parsedProgram = parseArgo(inputFileName_p)
	
	if (*printASTasGraphViz_p) {
		//listener.printASTnodes("rawWithText")
		parsedProgram.printASTnodes("dotShort")
	}
}
