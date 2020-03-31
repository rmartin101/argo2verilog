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

`ifdef NEGRESET
  `define RESET (~(rst))
`else
  `define RESET (rst)
`endif 

parameter MAX_CYCLES = 100;

   reg clk;
   reg rst;
   reg [DATA_WIDTH-1:0] bench_

initial begin
   // this module uses synchronous resets 
   // set the clock low and reset high to hold the system in the ready-to-reset state 
   clk = 0;
   rst = 1;
   #10;
   clk = 1;   // transitioning the clock lo to -high with reset high should reset everything 
   #10;
   rst = 0;  // pull reset and clock low 
   clk =0;   // hold clock low for a while
   bench_dataout = 0x33333;
   #10;      // let the clock go after this
end // initial 

   
argo_3stage STAGETEST (
    .clk(clk),
    .rst(rst),		 
    .ivalid(bench_ovalid),
    .iready(bench_oready),
    .ovalid(bench_ivalid),
    .oready(bench_iready),
    .datain(bench_dataout),
    .dataout(bench_datain)
);

   
/* clock control for the test bench */   
always begin 
  #1 clk = !clk ; 
end 

   /* *********** cycle counter ***********************/  
always @(posedge clk) begin
   if (`RESET) begin
      cycle_count <= 0;
   end
   else begin
      cycle_count <= cycle_count + 1 ;
   end
end
