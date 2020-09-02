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
			if  (len(cNode.successors_taken) > 0) {
				fmt.Fprintf(out," \t reg %s ; \n",cNode.cannName + "_taken" )				
			}
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
	fmt.Fprintf(out," \t clock = 0 ; \n ")
	fmt.Fprintf(out," \t rst = 0 ; \n ")
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
	fmt.Fprintf(out,"always @(posedge clock) begin \n")
	
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
	var sNode *StatementNode
	var pNode *ParseNode
	var sourceCode string
	
	out = parsedProgram.outputFile
	
	fmt.Fprintf(out,"// -------- Data Flow Section  ---------- \n")
	for _, vNode := range(parsedProgram.varNodeList) {
		fmt.Fprintf(out,"always @(posedge clock) begin // dataflow for variable %s \n", vNode.sourceName)
		fmt.Fprintf(out,"\t if `RESET begin \n ")
		fmt.Fprintf(out,"\t \t %s <= 0 ;  \n ",vNode.sourceName )
		fmt.Fprintf(out," \t end \n")
		fmt.Fprintf(out," \t else begin \n")			
		for i, cNode := range vNode.cfgNodes {
			sNode = cNode.statement
			pNode = sNode.parseDef 
			sourceCode = pNode.sourceCode
			// Fixme: Need to parse the expression and get the readvars
			sourceCode = strings.Replace(sourceCode,"=","<=",1)
			
			
			if i == 0 { fmt.Fprintf(out," \t \t if ( %s == 1 ) begin \n", cNode.cannName);
			} else {
				fmt.Fprintf(out," if ( %s == 1 ) begin \n", cNode.cannName);
			}
			fmt.Fprintf(out," \t \t \t %s ; \n", sourceCode)
			fmt.Fprintf(out," \t \t end \n")
			fmt.Fprintf(out," \t \t else ")
				
		}
		
		fmt.Fprintf(out," \t \t begin \n" )
		fmt.Fprintf(out," \t \t \t %s <= %s ; \n", vNode.sourceName,vNode.sourceName);
		fmt.Fprintf(out," \t \t end \n")
		fmt.Fprintf(out," \t end \n")		
		fmt.Fprintf(out,"end \n")
		
	}
}

/* ***************************************************** */
// ouput the control flow section 
func OutputControlFlow(parsedProgram *argoListener) {
	var out *os.File
	var entryClauses []string
	var allClauses string
	var cName string
	var stmtNode,testNode *StatementNode
	var pNode  *ParseNode 
	var condition string
	var debugFlags uint64 
	var DBG_CONTROL_MASK uint64 
	
	out = parsedProgram.outputFile
	debugFlags = parsedProgram.debugFlags
	DBG_CONTROL_MASK = 0x1
	
	fmt.Fprintf(out, "// -------- Control Flow Section  ---------- \n")

	for i, cNode := range(parsedProgram.controlFlowGraph) {

		// The start node gets its own clause 
		if (i == 0 ) {
			fmt.Fprintf(out,"always @(posedge clock) begin // control for %s \n",cNode.cannName)
			fmt.Fprintf(out,"\t if `RESET begin \n ")
			fmt.Fprintf(out,"\t \t %s <= 1 ; \n ", cNode.cannName)
			fmt.Fprintf(out,"\t end else begin \n ")
			fmt.Fprintf(out," \t \t " + cNode.cannName + " <=  0 ; \n")
			fmt.Fprintf(out," \t end \n ")				
			fmt.Fprintf(out,"end \n")
			continue
		}


		entryClauses = make([]string,0) 
		allClauses = ""
		cName = cNode.cannName 
		// if there must be predecessors for the control node to be reachable 
		if  ( len(cNode.predecessors) > 0) || (len(cNode.predecessors_taken) > 0) {

			// eos nodes from break/continue statements do not have a predecessor
			if ( len(cNode.predecessors) > 0 )  {
				if (len(cNode.predecessors_taken) == 0) {
					if (cNode.predecessors[0] == nil) {
						continue 
					}
				}
			}

			fmt.Fprintf(out,"always @(posedge clock) begin // control for %s \n",cNode.cannName)	

			fmt.Fprintf(out,"\t if `RESET begin \n ")
			
			fmt.Fprintf(out,"\t \t %s <= 0 ; \n ", cNode.cannName)

			if (cNode.cfgType == "ifTest") || (cNode.cfgType == "forCond" ) {
				fmt.Fprintf(out,"\t \t %s <= 0 ; \n ", cNode.cannName + "_taken" )
			}
			
			fmt.Fprintf(out,"\t end else begin \n ")
			
			for _, pred := range cNode.predecessors {
 				entryClauses = append(entryClauses,"( " + pred.cannName + " == 1 )" )
			}
			
			for _, p_taken := range cNode.predecessors_taken {
				entryClauses = append(entryClauses,"( " + p_taken.cannName + "_taken == 1 )") 
			}

			
			last := len(entryClauses)-1
			for j, clause := range entryClauses  {
				allClauses = allClauses + clause
				if  ( j < last )  {
					allClauses = allClauses + " || "
				}
			}

			fmt.Fprintf(out," \t \t if ( " + allClauses +  " ) begin \n")

			switch cNode.cfgType { 
			case "ifTest":
				stmtNode = cNode.statement
				testNode = stmtNode.ifTest
				pNode = testNode.parseDef
				condition = pNode.sourceCode
				
				fmt.Fprintf(out," \t \t \t if %s begin \n ",condition)
				takenName := cName + "_taken"
				
				fmt.Fprintf(out," \t \t \t \t %s <= 1 ; %s <= 0 ; \n",takenName,cName)
				if  ((debugFlags & DBG_CONTROL_MASK) == DBG_CONTROL_MASK) {
					fmt.Fprintf(out, " \t \t $display(\"%%5d,%%s,%%4d, at control node %%s if_taken \",cycle_count,`__FILE__,`__LINE__,\"" + cName + "\" ) ; \n") ;
				}
				fmt.Fprintf(out," \t \t \t end \n")
				fmt.Fprintf(out," \t \t \t else begin \n")
				fmt.Fprintf(out," \t \t \t \t %s <= 0 ; %s <= 1 ; \n",takenName,cName)

				if  ((debugFlags & DBG_CONTROL_MASK) == DBG_CONTROL_MASK) {
					fmt.Fprintf(out, " \t \t $display(\"%%5d,%%s,%%4d, at control node %%s if_not_taken \",cycle_count,`__FILE__,`__LINE__,\"" + cName + "\" ) ; \n") ;
				}				
				fmt.Fprintf(out," \t \t \t end \n")
				fmt.Fprintf(out," \t \t end \n")				
				fmt.Fprintf(out," \t \t else begin \n")
				fmt.Fprintf(out," \t \t \t \t %s <= 0 ; %s <= 0 ; \n",takenName,cName)
				fmt.Fprintf(out," \t \t end \n")				
			case "forCond":
				if (cNode.subStmt != nil ) {
					stmtNode = cNode.subStmt
					pNode = stmtNode.parseDef
					condition = "( " + pNode.sourceCode + " ) "
				} else {
					condition = "( 1 == 1 )"
				}

				
				fmt.Fprintf(out," \t \t \t if %s begin \n ",condition)
				takenName := cName + "_taken"
				
			fmt.Fprintf(out," \t \t \t \t %s <= 1 ; %s <= 0 ; \n",takenName,cName)
				if  ((debugFlags & DBG_CONTROL_MASK) == DBG_CONTROL_MASK) {
					fmt.Fprintf(out, " \t \t $display(\"%%5d,%%s,%%4d, at control node %%s for_taken \",cycle_count,`__FILE__,`__LINE__,\"" + cName + "\" ) ; \n") ;
				}			
				fmt.Fprintf(out," \t \t \t end \n")
				fmt.Fprintf(out," \t \t \t else begin \n")
				fmt.Fprintf(out," \t \t \t \t %s <= 0 ; %s <= 1 ; \n",takenName,cName)
				if  ((debugFlags & DBG_CONTROL_MASK) == DBG_CONTROL_MASK) {
					fmt.Fprintf(out, " \t \t $display(\"%%5d,%%s,%%4d, at control node %%s for_not_taken \",cycle_count,`__FILE__,`__LINE__,\"" + cName + "\" ) ; \n") ;
				}
				fmt.Fprintf(out," \t \t \t end \n")
				fmt.Fprintf(out," \t \t end \n")				
				fmt.Fprintf(out," \t \t else begin \n")
				fmt.Fprintf(out," \t \t \t \t %s <= 0 ; %s <= 0 ; \n",takenName,cName)
				fmt.Fprintf(out," \t \t end \n")
				
			default:
				fmt.Fprintf(out," \t \t \t " + cName + " <=  1 ; \n")
				if cNode.cfgType == "finishNode" {
					fmt.Fprintf(out," \t \t \t $finish() ; \n" )
				}
				if  ((debugFlags & DBG_CONTROL_MASK) == DBG_CONTROL_MASK) {
					fmt.Fprintf(out, " \t \t $display(\"%%5d,%%s,%%4d, at control node %%s \",cycle_count,`__FILE__,`__LINE__,\"" + cName + "\" ) ; \n") ;
				}
				fmt.Fprintf(out," \t \t end \n ")
				fmt.Fprintf(out," \t \t else begin \n ")
				fmt.Fprintf(out," \t \t \t " + cName + " <=  0 ; \n" )
				fmt.Fprintf(out," \t \t end \n ")				
				
			}
			fmt.Fprintf(out,"\t end \n")
			fmt.Fprintf(out,"end // end posedge clock \n ")
			fmt.Fprintf(out,"\n")			
		}
		

	}
	
}
/* ***************************************************** */
func OutputCycleCounter(out *os.File) { 
	fmt.Fprintf(out,"// the cycle counter for performance and debugging \n")
	fmt.Fprintf(out,"always @(posedge clock) begin \n")
	fmt.Fprintf(out," \t if `RESET begin \n")
	fmt.Fprintf(out," \t \t cycle_count <= 0; \n")
	fmt.Fprintf(out," \t    end    \n")
	fmt.Fprintf(out," \t    else begin \n")
	fmt.Fprintf(out," \t \t cycle_count <= cycle_count + 1 ; \n")
	fmt.Fprintf(out," \t    end \n")
	fmt.Fprintf(out," end \n")
}
func OutputVerilog(parsedProgram *argoListener) {
	var out *os.File
	// out := parsedProgram.outputFile
	out = parsedProgram.outputFile 
	
	fmt.Fprintf(out,"module %s(clock, rst);\n",parsedProgram.moduleName)
	fmt.Fprintf(out,"input clock;  // clock x1 \n") 
	fmt.Fprintf(out,"input rst;   // reset. Can set to positve or negative\n")
	fmt.Fprintf(out,"\n")
	
	fmt.Fprintf(out,"\n `define RESET (rst) \n")

	fmt.Fprintf(out,"\n")
	
	OutputVariables(parsedProgram)

	//OutputInitialization(parsedProgram)

	OutputIO(parsedProgram)

	OutputDataflow(parsedProgram)

	OutputControlFlow(parsedProgram)

	OutputCycleCounter(out)
	
	fmt.Fprintf(out,"endmodule\n")
}


