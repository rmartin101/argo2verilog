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

// output a very simple test-bench program that starts main
// with no parameters 
func OutputTestBench(parsedProgram *argoListener, max_cycles int) {
	var out *os.File
	out = parsedProgram.outputFile

	fmt.Fprintf(out,"module generic_bench(); \n")

	fmt.Fprintf(out," \t parameter MAX_CYCLES = %d; \n",max_cycles)
	fmt.Fprintf(out," \t reg clk;  // clock \n")
	fmt.Fprintf(out," \t reg rst;   // reset\n")
	fmt.Fprintf(out," \t reg start;  // start the main program 	\n")
	fmt.Fprintf(out," \t reg [31:0]  cycle_count;\n")
	fmt.Fprintf(out," \n")	
	fmt.Fprintf(out," \t main MAIN (\n")
	fmt.Fprintf(out," \t \t .clock(clk), \n")
	fmt.Fprintf(out," \t \t .rst(rst), \n")
	fmt.Fprintf(out," \t \t .start(start)\n")
	fmt.Fprintf(out," \t );\n")
	fmt.Fprintf(out," \n")	
	fmt.Fprintf(out," \t initial begin\n")
	fmt.Fprintf(out," \t \t clk = 0;  // force both reset and clock low \n")
	fmt.Fprintf(out," \t \t rst = 0; \n")
	fmt.Fprintf(out," \t \t #1; \n")
	fmt.Fprintf(out," \t \t rst = 1;  // pull reset and clock high, which generates a posedge clock and reset \n")
	fmt.Fprintf(out," \t \t clk = 1; \n")
	fmt.Fprintf(out," \t \t #1; \n")
	fmt.Fprintf(out," \t \t rst = 0;  // pull reset and clock low, then let clock run \n")
	fmt.Fprintf(out," \t \t clk = 0; \n")
	fmt.Fprintf(out," \t \t #1; \n")
	fmt.Fprintf(out," \t \t start = 1; // start the main function \n")
	fmt.Fprintf(out," \t \t clk = 1; \n")	
	fmt.Fprintf(out," \t \t #1; \n")
	fmt.Fprintf(out," \t \t start = 0; // start the main function \n")
	fmt.Fprintf(out," \t \t clk = 0; \n")	
	fmt.Fprintf(out," \t end // initial \n")
	fmt.Fprintf(out," \n")	
	fmt.Fprintf(out," \t /* clock control for the test bench */   \n")
	fmt.Fprintf(out," \t always begin \n")
	fmt.Fprintf(out," \t \t #6; // wait to run clock after start bit is set \n")	
	fmt.Fprintf(out," \t \t #1 clk = !clk ; \n")
	fmt.Fprintf(out," \t end \n")
	fmt.Fprintf(out," \n")	
	fmt.Fprintf(out," \t /* clock to end the simulation if we go too far  */ \n")
	fmt.Fprintf(out," \t always @(posedge clk) begin \n")
	fmt.Fprintf(out," \t \t if ( rst == 1 )  begin \n")
	fmt.Fprintf(out," \t \t \t cycle_count <= 0; \n")
	fmt.Fprintf(out," \t \t end else begin \n")
	fmt.Fprintf(out," \t \t \t if (cycle_count > MAX_CYCLES) begin \n")
	fmt.Fprintf(out," \t \t \t \t $finish(); \n")
	fmt.Fprintf(out," \t \t \t end else begin \n")
	fmt.Fprintf(out," \t \t \t \t cycle_count <= cycle_count + 1 ; \n")
	fmt.Fprintf(out," \t \t \t end \n")
	fmt.Fprintf(out," \t \t end \n")
	fmt.Fprintf(out," \t end \n")
	fmt.Fprintf(out," \n")	
	fmt.Fprintf(out,"endmodule // generic_bench   \n")
}

/* ***************************************************** */
func OutputVariables(parsedProgram *argoListener,funcName string) {

	// variable seciion 
	var out *os.File
	var numVnodes int
	
	out = parsedProgram.outputFile
	fmt.Fprintf(out,"// -------- Variable Section  ----------\n")
	fmt.Fprintf(out,"// --- User Variables ---- \n ")

	// count the number of nodes, if zero, do not output anything
	// FIXME: add count by function module name
	numVnodes = 0
	for _, vNode := range(parsedProgram.varNodeList) {
		if (vNode.funcName == funcName) {
			numVnodes ++ ;
		}
	}
	if (numVnodes == 0) {
		return; 
	}
	
	for _, vNode := range(parsedProgram.varNodeList) {

		// only print out variables names that match the current function 
		if (vNode.funcName == funcName) { 
			if vNode.goLangType == "numeric" {
				fmt.Fprintf(out," \t reg signed [%d:0] %s ; \n", vNode.numBits-1, vNode.sourceName)
			} else if vNode.primType == "array" {
			
			}
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

		if (cNode.statement.funcName == funcName) { 
			if ( (len(cNode.predecessors) > 0) || (len(cNode.predecessors_taken) >0) ) {
				fmt.Fprintf(out," \t reg %s ; \n",cNode.cannName)
				if  (len(cNode.successors_taken) > 0) {
					fmt.Fprintf(out," \t reg %s ; \n",cNode.cannName + "_taken" )				
				}
			}
		}
	}
	
}

/* ***************************************************** */
// ouput the initialization section for simulation 
func OutputInitialization(parsedProgram *argoListener,funcName string) {
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
func OutputIO(parsedProgram *argoListener,funcName string) {
	var out *os.File
	var stmt *StatementNode
	var pNode *ParseNode
	var sourceCode string
	var numCnodes int
	
	out = parsedProgram.outputFile
	
	fmt.Fprintf(out,"// -------- I/O Section  ---------- \n")


	// count the number of printf nodes, if zero, do not output anything
	// FIXME: add count by function module name
	numCnodes = 0
	for _, cNode := range(parsedProgram.controlFlowGraph) {
		if (cNode.statement.funcName == funcName) {
			if (cNode.cfgType == "expression" ) {
				stmt = cNode.statement
				pNode = stmt.parseDef
				sourceCode = pNode.sourceCode
				if strings.Contains(sourceCode,"fmt.Printf") {
					numCnodes ++ ;
				}
			}
		}
	}
	if (numCnodes == 0) {
		return; 
	}
	
	fmt.Fprintf(out,"always @(posedge clock) begin \n")
	
	for _, cNode := range(parsedProgram.controlFlowGraph) {

		if (cNode.statement.funcName == funcName) { 
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
	}
	fmt.Fprintf(out,"end \n")

	
}

/* ***************************************************** */
// ouput the data flow section 
func OutputDataflow(parsedProgram *argoListener,funcName string) {
	var out *os.File
	var sMainNode,sSubNode,sNode *StatementNode
	var pNode *ParseNode
	var sourceCode string
	var debugFlags uint64 
	var DBG_CONTROL_MASK uint64
	
	out = parsedProgram.outputFile

	debugFlags = parsedProgram.debugFlags
	DBG_CONTROL_MASK = 0x1
	
	fmt.Fprintf(out,"// -------- Data Flow Section  ---------- \n")
	for _, vNode := range(parsedProgram.varNodeList) {


		if (vNode.funcName == funcName) { 
		
			fmt.Fprintf(out,"always @(posedge clock) begin // dataflow for variable %s \n", vNode.sourceName)
			fmt.Fprintf(out,"\t if `RESET begin \n ")
			fmt.Fprintf(out,"\t \t %s <= 0 ;  \n ",vNode.sourceName )
			fmt.Fprintf(out," \t end \n")
			fmt.Fprintf(out," \t else begin \n")			
			for i, cNode := range vNode.cfgNodes {
				sMainNode = cNode.statement
				sSubNode = cNode.subStmt 
				// if a cfg node has a sub-node, it is an if or for conditional/post 
				if (sSubNode != nil) {
					sNode = sSubNode 				
				} else {
					sNode = sMainNode 
				}
				pNode = sNode.parseDef 
				sourceCode = pNode.sourceCode

				// Fixme: Need to parse the expression and get the readvars

				sourceCode = expressionToString(pNode)
				
				sourceCode = strings.Replace(sourceCode,"=","<=",1)

				
				if i == 0 {
					fmt.Fprintf(out," \t \t if ( %s == 1 ) begin \n", cNode.cannName);
				} else {
					fmt.Fprintf(out," if ( %s == 1 ) begin \n", cNode.cannName);
				}
				fmt.Fprintf(out," \t \t \t %s ; \n", sourceCode)

				if  ((debugFlags & DBG_CONTROL_MASK) == DBG_CONTROL_MASK) {
					fmt.Fprintf(out, " \t \t $display(\"a2gDbg,%%5d,%%s,%%4d, dataflow %%s \",cycle_count,`__FILE__,`__LINE__,\"" + sourceCode + "\" ) ; \n") ;
				}
			
				fmt.Fprintf(out," \t \t end \n")
				fmt.Fprintf(out," \t \t else ")
				
			}
		
			fmt.Fprintf(out," begin \n" )
			fmt.Fprintf(out," \t \t \t %s <= %s ; \n", vNode.sourceName,vNode.sourceName);
			fmt.Fprintf(out," \t \t end \n")
			fmt.Fprintf(out," \t end \n")		
			fmt.Fprintf(out,"end \n")
		
		}
	}
}

/* ***************************************************** */
// Ouput the control flow section 
func OutputControlFlow(parsedProgram *argoListener,funcName string) {
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
		if (i == 0 ) && (funcName == "main") {
			fmt.Fprintf(out,"\t always @(posedge clock) begin // control for %s \n",cNode.cannName)
			fmt.Fprintf(out,"\t \t if `RESET begin \n ")
			fmt.Fprintf(out,"\t \t \t %s <= 0 ; \n ", cNode.cannName)
			fmt.Fprintf(out,"\t \t end \n ")
			fmt.Fprintf(out,"\t \t else if (start == 1) begin \n ")
			fmt.Fprintf(out,"\t \t \t " + cNode.cannName + " <=  1 ; \n")
			fmt.Fprintf(out,"\t \t end \n ")						
			fmt.Fprintf(out,"\t \t else begin\n ")			
			fmt.Fprintf(out,"\t \t \t "  + cNode.cannName + " <=  0 ; \n")
			fmt.Fprintf(out,"\t \t end \n ")
			fmt.Fprintf(out,"\t end \n ")				
			continue
		}
		
		if (cNode.statement.funcName == funcName) {
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
						fmt.Fprintf(out, " \t \t $display(\"a2gDbg,%%5d,%%s,%%4d, at control node %%s if_taken \",cycle_count,`__FILE__,`__LINE__,\"" + cName + "\" ) ; \n") ;
					}
					fmt.Fprintf(out," \t \t \t end \n")
					fmt.Fprintf(out," \t \t \t else begin \n")
					fmt.Fprintf(out," \t \t \t \t %s <= 0 ; %s <= 1 ; \n",takenName,cName)

					if  ((debugFlags & DBG_CONTROL_MASK) == DBG_CONTROL_MASK) {
						fmt.Fprintf(out, " \t \t $display(\"a2gDbg,%%5d,%%s,%%4d, at control node %%s if_not_taken \",cycle_count,`__FILE__,`__LINE__,\"" + cName + "\" ) ; \n") ;
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
						fmt.Fprintf(out, " \t \t $display(\"a2gDbg,%%5d,%%s,%%4d, at control node %%s for_taken \",cycle_count,`__FILE__,`__LINE__,\"" + cName + "\" ) ; \n") ;
					}			
					fmt.Fprintf(out," \t \t \t end \n")
					fmt.Fprintf(out," \t \t \t else begin \n")
					fmt.Fprintf(out," \t \t \t \t %s <= 0 ; %s <= 1 ; \n",takenName,cName)
					if  ((debugFlags & DBG_CONTROL_MASK) == DBG_CONTROL_MASK) {
						fmt.Fprintf(out, " \t \t $display(\"a2gDbg,%%5d,%%s,%%4d, at control node %%s for_not_taken \",cycle_count,`__FILE__,`__LINE__,\"" + cName + "\" ) ; \n") ;
				}
					fmt.Fprintf(out," \t \t \t end \n")
					fmt.Fprintf(out," \t \t end \n")				
					fmt.Fprintf(out," \t \t else begin \n")
					fmt.Fprintf(out," \t \t \t \t %s <= 0 ; %s <= 0 ; \n",takenName,cName)
					fmt.Fprintf(out," \t \t end \n")
					
				default:
					fmt.Fprintf(out," \t \t \t " + cName + " <=  1 ; \n")
					if  ((debugFlags & DBG_CONTROL_MASK) == DBG_CONTROL_MASK) {
						fmt.Fprintf(out, " \t \t $display(\"a2gDbg,%%5d,%%s,%%4d, at control node %%s \",cycle_count,`__FILE__,`__LINE__,\"" + cName + "\" ) ; \n") ;
					}
					if cNode.cfgType == "finishNode" {
						fmt.Fprintf(out," \t \t \t $finish() ; \n" )
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
}
/* ***************************************************** */
func OutputCycleCounter(out *os.File,funcName string) { 
	fmt.Fprintf(out,"\t // the cycle counter for performance and debugging \n")
	fmt.Fprintf(out,"\t always @(posedge clock) begin \n")
	fmt.Fprintf(out," \t \t if `RESET begin \n")
	fmt.Fprintf(out," \t \t \t cycle_count <= 0; \n")
	fmt.Fprintf(out," \t \t    end    \n")
	fmt.Fprintf(out," \t \t    else begin \n")
	fmt.Fprintf(out," \t \t \t cycle_count <= cycle_count + 1 ; \n")
	fmt.Fprintf(out," \t \t    end \n")
	fmt.Fprintf(out," \t end \n")
}

func OutputVerilog(parsedProgram *argoListener,genTestBench bool,max_cycles int) {
	var out *os.File
	var funcNode *FunctionNode
	var funcName string
	
	// out := parsedProgram.outputFile
	out = parsedProgram.outputFile 

	if (genTestBench)  {
		OutputTestBench(parsedProgram,max_cycles)
	}

	// each Go function maps to a verilog Module 
	for _, funcNode = range parsedProgram.funcNodeList {

		funcName = funcNode.funcName 
		fmt.Fprintf(out,"module %s(clock, rst,start);\n",funcName)
		fmt.Fprintf(out,"\t input clock;  // clock x1 \n") 
		fmt.Fprintf(out,"\t input rst;    // reset. Can set to positve or negative\n")
		fmt.Fprintf(out,"\t input start;  // start the function \n")
		fmt.Fprintf(out,"\n")
	
		fmt.Fprintf(out,"\n \t `define RESET (rst) \n")

		fmt.Fprintf(out,"\n")
		
		OutputVariables(parsedProgram,funcName)

		//OutputInitialization(parsedProgram)

		OutputIO(parsedProgram,funcName)
		
		OutputDataflow(parsedProgram,funcName)
		
		OutputControlFlow(parsedProgram,funcName)

		OutputCycleCounter(out,funcName)
		
		fmt.Fprintf(out,"endmodule \n")
		fmt.Fprintf(out,"// ----------------------------------------------- \n")
	}
		

}


