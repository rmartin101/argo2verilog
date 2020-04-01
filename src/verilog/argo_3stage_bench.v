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

`ifdef NEGRESET
  `define RESET (~(rst))
`else
  `define RESET (rst)
`endif 

module argo_3stage_bench();

   parameter MAX_CYCLES = 25;

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
       .clk(clk),
       .rst(rst),		 
       .ivalid(bench_ovalid),
       .iready(bench_iready),
       .ovalid(bench_ivalid),
       .oready(bench_oready),
       .datain(bench_dataout),
       .dataout(bench_datain)
   );
   assign bench_iready = bench_iready_reg;
   
   initial begin
      // this module uses synchronous resets 
      // set the clock low and reset high to hold the system in the ready-to-reset state
      bench_ovalid =0;
      clk = 0;
      rst = 1;
      #10;
      clk = 1;   // transitioning the clock lo to -high with reset high should reset everything 
      #10;
      bench_dataout = 32'h0000001;
      rst = 0;  // pull reset and clock low 
      clk =0;   // hold clock low for a while
      #10;      // let the clock go after this
   end // initial 

   
   /* *********** data writer ***********************/
   always @(posedge clk) begin
      if (bench_oready == 1) begin
	 // this case statement is the input driver organized by the specific cycle count 
	 case (cycle_count)
	   1 : begin 
	      bench_dataout <= 'h55;
	      bench_ovalid <= 1;
	      $display("Sending %d to pipeline cycle %d",bench_dataout,cycle_count);
	   end 
	   2: begin 
	      bench_dataout <= 'h25;
	      bench_ovalid <= 1;
	      $display("Sending %d to pipeline cycle %d",bench_dataout,cycle_count);
	   end
	   default: begin 
	      bench_dataout <= 0;
	      bench_ovalid <= 0;
	   end
	 endcase
      end else begin // if (oready == 0, the pipe is not ready to accept data )
	bench_dataout <= 0;
	bench_ovalid <= 0;	 
     end // else: !if(bench_oready == 1)
   end // always @ (posedge clk)

   /* *********** data reader ***********************/
   // we always accept data    
   always @(posedge clk) begin
      bench_iready_reg <= 1;  // act as infinite sink
      if (bench_ivalid == 1) begin
	 bench_datain_reg <= bench_datain;
      end else begin
	 bench_iready_reg <=1;
      end
   end
   
   /* *********** cycle counter ***********************/  
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
