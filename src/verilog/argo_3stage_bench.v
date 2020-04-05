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

//`define `POSRESET
`define NEGRESET
  
`ifdef NEGRESET
  `define RESET (~(rst))
`else
  `define RESET (rst)
`endif 

module argo_3stage_bench();

   parameter MAX_CYCLES = 50;

   reg clk;  // lock 
   reg rst;   // reset 
   reg [31:0]  cycle_count;
   
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
      // the 3 stage bench module uses synchronous resets 
      // set the clock low and reset high to hold the system in the ready-to-reset state
      bench_ovalid =0;
      bench_dataout = 'h55;
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
      if (bench_oready == 1) begin
	 // this case statement is the input driver organized by the specific cycle count
	 if ( (cycle_count >=0) && (cycle_count < 10) ) begin 
	      bench_dataout = 'h25;
	      bench_ovalid = 1;
	      $display("%5d,%s,%4d,Sending h%h to pipeline",cycle_count,`__FILE__,`__LINE__,bench_dataout);
	   end else if ((cycle_count >=10) && (cycle_count < 20)) begin 
	     bench_dataout = 'h55;
	     bench_ovalid = 1;
	     $display("%5d,%s,%4d,Sending h%h to pipeline",cycle_count,`__FILE__,`__LINE__,bench_dataout);
	   end else if ((cycle_count >=20) && (cycle_count < 30)) begin 
	      bench_dataout = 'h19700328;
	      bench_ovalid = 1;
	      $display("%5d,%s,%4d,Sending h%h to pipeline",cycle_count,`__FILE__,`__LINE__,bench_dataout);
	   end else begin
	      bench_dataout = cycle_count;
	      bench_ovalid = 1;
	   end
	end // if (bench_oready == 1)
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
