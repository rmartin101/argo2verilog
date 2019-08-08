package main

import (
	"strings"
	"strconv"
	"fmt"
	"os"
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

type argoListener struct {
	*parser.BaseArgoListener
	stack []int
	recog antlr.Parser
	ProgramLines []string // the program as a list of strings, one string per line
	node2ID map[interface{}]int //  a map of the AST node pointers to small integer ID mapping
	nextID int
	astNodeList []*astNode
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


// print all the nodes
func (l *argoListener) printASTnodes(outputStyle string) {

	var nodeStr string // name of the AST node 
	
	sort.Slice(l.astNodeList, func(i, j int) bool {
		return l.astNodeList[i].id < l.astNodeList[j].id
	})

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


// EnterStart tries to crawl the whole tree 
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
	l.addASTnode(&root)
	
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
		fmt.Printf("getFileLines Error at line \n")
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
	


// ProgramLines is a global variable that holds the text listing of the program
// as a list of lines, where each line is a string



// parseargo takes a string expression and returns the evaluated resuacklt.
func parseargo(fname string) int {

	var err error
	var listener argoListener
	
	input, _ := antlr.NewFileStream(fname)
	lexer := parser.NewArgoLexer(input)
	stream := antlr.NewCommonTokenStream(lexer,0)
	
	p := parser.NewArgoParser(stream)

	listener.recog = p
	progLines, err2 := getFileLines(os.Args[1])
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
	

	//listener.printASTnodes("rawWithText")
	listener.printASTnodes("dotShort")
	
	return 0
}

func main() {
	var r int
	
	r = parseargo(os.Args[1])
	fmt.Printf(" results: %d",r)

	
}
