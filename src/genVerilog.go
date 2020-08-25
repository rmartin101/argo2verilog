/* Argo to Verilog Compiler 
    (c) 2020, Richard P. Martin and contributers 
    
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


/* Routines to generate the Verilog executable */

package main


import (
	"fmt"
	"os"
	"strings"
	"regexp"
)

/* ***************************************************** */
func OutputVariables(parsedProgram *argoListener) {

	// variable seciion 
	var out *os.File
	out = parsedProgram.outputFile
	fmt.Fprintf(out,"// -------- Variable Section  ----------\n")
	fmt.Fprintf(out,"// --- User Variables ---- \n ")	
	for _, vNode := range(parsedProgram.varNodeList) {
		if vNode.goLangType == "numeric" {
			fmt.Fprintf(out," \t reg signed [%d:0] %s ; \n", vNode.numBits-1, vNode.sourceName)
		} else if vNode.primType == "array" {
			
		}
	}
	fmt.Fprintf(out,"// --- Control Bits ---- \n")
	fmt.Fprintf(out," \t reg clk ; \n")
	fmt.Fprintf(out," \t reg [63:0] cycle_count ; \n")

	
	l := len(parsedProgram.controlFlowGraph)
	if ( l == 0 ) {
		fmt.Printf("Error: zero control flow nodes at line %d %s \n", l, _file_line_())
		return ;
	}
	
	fmt.Fprintf(out," \t reg %s ; \n",parsedProgram.controlFlowGraph[0].cannName)
	for _, cNode := range(parsedProgram.controlFlowGraph) {
		if ( (len(cNode.predecessors) > 0) || (len(cNode.predecessors_taken) >0) ) {
			fmt.Fprintf(out," \t reg %s ; \n",cNode.cannName)
		}
	}
	
}

/* ***************************************************** */
// ouput the initialization section for simulation 
func OutputInitialization(parsedProgram *argoListener) {
	var out *os.File
	out = parsedProgram.outputFile
	
	fmt.Fprintf(out,"// -------- Initialization Section  ---------- \n")
	fmt.Fprintf(out,"initial begin \n")
	fmt.Fprintf(out," \t clk = 0 ; \n ")
	fmt.Fprintf(out," \t cycle_count = 0 ; \n")
	fmt.Fprintf(out," \t %s = 1 ; \n",parsedProgram.controlFlowGraph[0].cannName)
	fmt.Fprintf(out,"end \n")
}

/* ***************************************************** */
// ouput the I/O section for simulation
// right now just change the printfs to $display statements 
func OutputIO(parsedProgram *argoListener) {
	var out *os.File
	var stmt *StatementNode
	var pNode *ParseNode
	var sourceCode string

	out = parsedProgram.outputFile
	
	fmt.Fprintf(out,"// -------- I/O Section  ---------- \n")
	fmt.Fprintf(out,"always @(posedge clk) begin \n")
	
	for _, cNode := range(parsedProgram.controlFlowGraph) {
		if (cNode.cfgType == "expression" ) {
			stmt = cNode.statement
			pNode = stmt.parseDef
			sourceCode = pNode.sourceCode
			if strings.Contains(sourceCode,"fmt.Printf") {
				exp := regexp.MustCompile(`\(.*\)`)
				innerExp := exp.FindString(sourceCode)
				displayStr := "$write" + innerExp + "; "
				fmt.Fprintf(out," \t if (%s == 1) begin \n",cNode.cannName)
				fmt.Fprintf(out," \t \t %s \n",displayStr)
				fmt.Fprintf(out," \t end \n")
			}
		}
	}
	fmt.Fprintf(out,"end \n")

	
}

/* ***************************************************** */
// ouput the data flow section 
func OutputDataflow(parsedProgram *argoListener) {
	var out *os.File
	out = parsedProgram.outputFile
	
	fmt.Fprintf(out,"// -------- Data Flow Section  ---------- \n")
	for _, vNode := range(parsedProgram.varNodeList) {
		for _, cNode := range vNode.cfgNodes {
			fmt.Fprintf(out," at %s writevar %s \n",cNode.cannName,vNode.sourceName)
		}
	}
	
}

/* ***************************************************** */
// ouput the control flow section 
func OutputControlFlow(parsedProgram *argoListener) {
	var out *os.File
	out = parsedProgram.outputFile
	
	fmt.Fprintf(out, "// -------- Control Flow Section  ---------- \n")
}
/* ***************************************************** */

func OutputVerilog(parsedProgram *argoListener) {
	var out *os.File
	// out := parsedProgram.outputFile
	out = parsedProgram.outputFile 
	
	fmt.Fprintf(out,"module %s();\n",parsedProgram.moduleName)
	
	OutputVariables(parsedProgram)

	OutputInitialization(parsedProgram)

	OutputIO(parsedProgram)

	OutputDataflow(parsedProgram)

	OutputControlFlow(parsedProgram)

	fmt.Fprintf(out,"endmodule\n")
}


