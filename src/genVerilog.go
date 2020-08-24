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
	
}

/* ***************************************************** */
// ouput the I/O section for simulation 
func OutputIO(parsedProgram *argoListener) {
	
}

/* ***************************************************** */
// ouput the data flow section 
func OutputDataflow(parsedProgram *argoListener) {
	
}

/* ***************************************************** */
// ouput the control flow section 
func OutputControlFlow(parsedProgram *argoListener) {
	
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


