/* Argo to Verilog Compiler: Testbench for the Verilog FIFO and RAM Templates 
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

/* This test bench creates 2 FIFOs and 3 control-bit loops, which model a set of 3 go-routines 
 * connected by channels. The first loop pushs N values into the first FIFO using a variable Y1. The second 
 * control loop reads from FIFO 1 into variable X1 and writes it FIFO 2. The third loop reads from 
 * FIFO 2 into variable Z1
*/

/* the dataflow model for this module follow the openCL library avalon interface:
 *
 *                 Upstream module    Data
 * 
 *           bench_oready bench_ovalid bench_dataout
 *                 ^            |       |
 *         --------|------------V-------V-------  
 *         | oready|         ivalid     data   |
 *         |       |                    input  |
 *         |  This module (3 stage pipe)       |
 *         |                           data    |
 *         |    iready       ovalid    out     |
 *         ------- ^------------|-------|------|
 *                 |            V       V      
 *          bench_iready bench_ivalid  bench_datain
 * 
 *                  Downstream module 
 * 
 * The middle module sets oready. If ivalid is high, the module must latch the data. 
 */

/* This test bench works is modeled on an openCL FPGA library. 
 * a vector/array of inputs and outputs is used. Each element is an input and the results of 
 * that input are stored in an output. E.g. output[i] = FPGA(input[i]). 
 * This code uses a mostly async interface, although there is a 1 cycle delay to 
 * latch the input once we set the ivalid line high
 */

//`define `POSRESET
`define NEGRESET 
  
`ifdef NEGRESET
  `define RESET (~(rst))
`else
  `define RESET (rst)
`endif 

module argo_3stage_bench();

   parameter MAX_CYCLES = 100;
   parameter INPUT_SIZE = 256;

   // test bench state machine parameters 
   parameter READY  =    0;  // going to send data 
   parameter LATCH =     1;  // one-cycle delay for pipeline to latch data 
   parameter WAITING =   2;  // waiting for the output 
   
   reg clk;  // lock 
   reg rst;   // reset 
   reg [31:0]  cycle_count;
   reg [7:0]   current_state;
    
   reg [7:0]  i;  // index for the input array 
   reg [7:0]  j;  // index for the output array
   
   reg [31:0]  input_values [INPUT_SIZE];
   reg [31:0]  output_values [INPUT_SIZE];
   
   // inputs to the pipeline
   reg bench_oready;
   reg bench_ovalid;
   reg [31:0] bench_dataout;

   // output from the pipeline 
   wire bench_iready;
   reg  bench_iready_reg;
   
   wire bench_ivalid;
   wire [31:0] bench_datain;
   reg  [31:0]  bench_datain_reg;
   
   argo_3stage STAGETEST (
       .clock(clk),
       .resetn(rst),		 
       .ivalid(bench_ovalid),
       .iready(bench_iready),
       .ovalid(bench_ivalid),
       .oready(bench_oready),
       .datain(bench_dataout),
       .dataout(bench_datain)
   );
   assign bench_iready = bench_iready_reg;
   
   initial begin
      clk = 0;  // force both reset and clock low 
      rst = 0;
      $display("%5d,%s,%4d,initial begin",cycle_count,`__FILE__,`__LINE__);      
      // the 3 stage bench module uses synchronous resets 
      // set the clock low and reset high to hold the system in the ready-to-reset state
      bench_ovalid =0;
      // populate the input vector 
      for (i = 0; i< 25 ; i = i + 1 ) begin
	 $display("%5d,%s,%4d, in loop",cycle_count,`__FILE__,`__LINE__,i);      
	  case (i)
	    0:  input_values[i] = 'h19700328;
	    1:  input_values[i] = 'h19700101;
	    2:  input_values[i] = 'h19700328;
	    3:  input_values[i] = 'h19700101;
	    10: input_values[i] = 'h19700328;
	    12: input_values[i] = 'h19700101;
	    default: input_values[i] = i % 7;
	   endcase  
      end	
      i = 0;
      j = 0;
      current_state = READY;
      $display("%5d,%s,%4d,current state %d",cycle_count,`__FILE__,`__LINE__,current_state);      
`ifdef POSRESET
      clk = 0;  // force both reset and clock low 
      rst = 0;
      #1;
      rst = 1;  // pull reset and clock high, which generates a posedge clock and reset 
      clk = 1; 
      #1;
      rst = 0;  // pull reset and clock low, then let clock run
      clk = 0;
`endif 
`ifdef NEGRESET
      clk = 0;  // force both reset and clock low 
      rst = 1;
      #1;
      rst = 0;  // pull reset and clock high, which generates a posedge clock and reset 
      clk = 1; 
      #1;
      rst = 1;  // pull reset and clock low, then let clock run
      clk = 0;
`endif 
      
   end // initial 

   
   /* *********** data writer ***********************/
   /* use block assignements to make things easier in the test bench */
   /* so everything happens by the end of the clock */
   always @(posedge clk) begin
      $display("%5d,%s,%4d,current state %d",cycle_count,`__FILE__,`__LINE__,current_state);
      case (current_state)
	READY : begin
	   if (bench_oready == 1) begin
	      bench_dataout = input_values[i];
	      bench_ovalid = 1;
	      bench_iready_reg = 1;
	      $display("%5d,%s,%4d,Sending h%h to pipeline",cycle_count,`__FILE__,`__LINE__,input_values[i]);
	      i = i + 1;
	      current_state = LATCH;
	   end else begin
	      $display("%5d,%s,%4d,Waiting Write to Pipeline",cycle_count,`__FILE__,`__LINE__);
	   end
	end // case: READY
	LATCH: begin
	   bench_ovalid = 1;
	   current_state = WAITING;
	   $display("%5d,%s,%4d, Delay latch into pipeline",cycle_count,`__FILE__,`__LINE__);
	end
	
	WAITING: begin
	   bench_ovalid = 0;  // must use a taking-turns protocol to get next input so set to zero 
	   if (bench_ivalid == 1) begin
	      output_values[j] = bench_datain;
	      j = j +1;
	      $display("%5d,%s,%4d, Received value from last stage h%h ",cycle_count,`__FILE__,`__LINE__,bench_datain);
	      current_state = READY;
	   end else begin
	      $display("%5d,%s,%4d, Waitng Read from Pipeline ",cycle_count,`__FILE__,`__LINE__);
	   end
	end
      endcase 

   end // always @ (posedge clk)

   /* *********** data reader ***********************/
   // The testbench always accepts data    
   always @(posedge clk) begin
      bench_iready_reg = 1;  // act as infinite sink
      if (bench_ivalid == 1) begin
	 bench_datain_reg = bench_datain;
	 $display("%5d,%s,%4d, read value from last stage h%h ",cycle_count,`__FILE__,`__LINE__,bench_datain);	 
      end else begin
	 bench_iready_reg =1;
      end
   end
   
   /* *********** cycle counter ***********************/
   /* We could use the $time primitive, but dont */
   
   always @(posedge clk) begin
      if (`RESET) begin
	 cycle_count <= 0;
      end
      else begin
	 cycle_count <= cycle_count + 1 ;
	 if (cycle_count > MAX_CYCLES) begin
	    $finish();
	 end
      end
   end

/* clock control for the test bench */   
   always begin 
      #1 clk = !clk ; 
   end 


endmodule // argo_3stage_bench
