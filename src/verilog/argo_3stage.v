/* Argo to Verilog Compiler: Test of a 3 stage pipeline with filter 
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


/* A 3-stage pipeline with a simple filter 
 * This tests the Verilog structure of the Argo to Verilog compiler 
 * The goal of this code to test on an actual board 
 * This code mimics the mapping of Go channels to Verilog 
 * 
 * Each pipeline stage is a state machine defined by a set of control bits using 1-hot encoding 
 * Stage 1 are control bits 00001 to 00008 (c_bit_00001-c_bit_00008)
 * Stage 2 are control bits 00101 to 00108 (c_bit_00101-c_bit_00108)
 * Stage 3 are control bits 00201 to 00208 (c_bit_00101-c_bit_00108)
 * FIFOs are used to connect the stages, instantiated as PIPE_1 and PIPE_2
 * The data-flow section contains the control for the variables 
 * X1 is a simulated Go variable that is read from data-in
 * X1 is the variable written to to pipe1 by stage 1 
 * 
 * Y1 is a simulated Go variable read by stage 2 from pipe1 and written to pipe2
 * Y1 is filtered, If Y1 == 0x19700328, it is changed to 0x20050823 
 * If Y1 == 0x19700101, it is changed to 0x20071224 
 * otherwize, Y1 is pushed through the pipeline as is
 * 
 * Z1 is a simulated Go variable that is read by stage 3 from pipe 2 
 * Z1 is then output to dataout
 */
 /* switch for positive vs negative resets */

/* the dataflow model for this module follow the openCL library:
 *
 *                 Upstream module    Data
 *                 ^            |     |
 *           oready|     ivalid |     |
 *                 |            V     V
 *            This module (3 stage pipe)
 *                  ^            |    |
 *            iready|     ovalid |   Data
 *                  |            V    V
 *                  Downstream module 
 */

`ifdef NEGRESET
 `define RESET (~(rst))
`else
 `define RESET (rst)
 `endif


module argo_3stage(clk,rst,ivalid,iready,ovalid,oready,datain,dataout);
   parameter PIPE_1_WIDTH = 32;
   parameter PIPE_1_ADDR_WIDTH = 4;
   parameter PIPE_2_WIDTH = 32;
   parameter PIPE_2_ADDR_WIDTH = 3;

   input clk;  // clock 
   input rst;   // reset. Can set to positve or negative 
   input ivalid;  // input valid 
   input iready;   // input ready 
   output ovalid;  // output valid 
   output oready;  // output ready 
   input [PIPE_1_WIDTH-1:0] datain;    // data input 
   output [PIPE_2_WIDTH-1:0] dataout);  // data output

   /* variables */
   reg [31: 0] X1;  // write into FIFO 1
   reg [31: 0] Y1;  // Read from FIFO 1
   reg [31: 0] Z1;  // write to FIFO 2

   /* control regs */
   reg [31:0] cycle_count ;  // cycle counter for performance and debugging

   /* control regs for the first control loop */
   reg c_bit_00000_start ;  // initial state control
   reg c_bit_00001;  // the control bits write into pipe 1
   reg c_bit_00002;
   reg c_bit_00003;
   reg c_bit_00004;
   reg c_bit_00005;
   reg c_bit_00006;   
   reg c_bit_00007;
   reg c_bit_00008;   

   /* control regs for the second control loop */
   // move data from pipe 1 to pipe 2
   reg c_bit_00101;  // these control read from pipe 1
   reg c_bit_00102;  // and write to pipe 2
   reg c_bit_00103;
   reg c_bit_00104;
   reg c_bit_00105;
   reg c_bit_00106;   
   reg c_bit_00107;
   reg c_bit_00108;

   /* control regs for third control loop */
   /* read from pipe 2 */
   reg c_bit_00201;  
   reg c_bit_00202;  
   reg c_bit_00203;
   reg c_bit_00204;
   reg c_bit_00205;
   reg c_bit_00206;   
   reg c_bit_00207;
   reg c_bit_00208;   

   /* control regs for the third control loop */
   
   // Pipe/channel 1 registers    
   reg [PIPE_1_WIDTH-1:0 ] pipe_1_write_data;
   wire [PIPE_1_WIDTH-1:0 ] pipe_1_read_data;
   reg pipe_1_rd_en_reg;
   reg pipe_1_wr_en_reg;   
   wire pipe_1_full;
   wire pipe_1_empty;

   reg  [PIPE_2_WIDTH-1:0 ] pipe_2_write_data;
   wire [PIPE_2_WIDTH-1:0 ] pipe_2_read_data;
   reg pipe_2_rd_en_reg;
   reg pipe_2_wr_en_reg;   
   wire pipe_2_full;
   wire pipe_2_empty;

/* channels */
argo_fifo #(.ADDR_WIDTH(PIPE_1_ADDR_WIDTH),.DATA_WIDTH(PIPE_1_WIDTH),.DEPTH(1<<PIPE_1_ADDR_WIDTH),.FIFO_ID(1)) PIPE_1 (
    .clk(clk),
    .rst(rst),		 
    .rd_en(pipe_1_rd_en_reg),
    .rd_data(pipe_1_read_data),
    .wr_en(pipe_1_wr_en_reg),
    .wr_data(pipe_1_write_data),
    .full(pipe_1_full),
    .empty(pipe_1_empty)
);
 
/* the 2nd channel */
argo_fifo #(.ADDR_WIDTH(PIPE_2_ADDR_WIDTH),.DATA_WIDTH(PIPE_2_WIDTH),.DEPTH(1<<PIPE_2_ADDR_WIDTH),.FIFO_ID(5)) PIPE_2 (
    .clk(clk),
    .rst(rst),		 
    .rd_en(pipe_2_rd_en_reg),
    .rd_data(pipe_2_read_data),
    .wr_en(pipe_2_wr_en_reg),
    .wr_data(pipe_2_write_data),
    .full(pipe_2_full),
    .empty(pipe_2_empty)
);

   
// ************ Data flow section ********************* */
always @(posedge clk) begin // Data flow for X1
   if `RESET begin
      X1 <=0;
   end else if (c_bit_00001 == 1) begin
	 X1 <= X1 + 1;
	 $display("incrementing X1 val %d at cycle %d",X1,cycle_count);
   end else begin
      X1 <= X1 ;      
   end
end
      

/**** channel 1 writer data flow section *********/
always @(posedge clk) begin // control for line c_bit_00001;
   if `RESET begin
      pipe_1_write_data <= 0;
      pipe_1_wr_en_reg <= 0;
   end 
   else if (c_bit_00003 == 1) begin
      pipe_1_write_data <= X1 ;
      pipe_1_wr_en_reg <= 1 ;
      $display("storing X1 into  pipe 1 val: %d cycle %d",X1,cycle_count);
   end else begin 
      pipe_1_write_data <= 0 ;
      pipe_1_wr_en_reg <= 0 ;
   end
end // always @ (posedge clk)
   
   
/**** channel 1 reader data  flow section ****/
always @(posedge clk) begin // data flow for read enable on FIFO 1
   if `RESET begin
      pipe_1_rd_en_reg <= 0 ;      
   end
   else if ((c_bit_00103 == 1) && (!(pipe_1_empty )))begin
      pipe_1_rd_en_reg <= 1 ;
      $display("enabling read on pipe 1 cycle %d",cycle_count);
      end else begin 
	 pipe_1_rd_en_reg <= 0 ;
   end
end

always @(posedge clk) begin // data flow for reads of the filo
   if `RESET begin
      Y1 <= 0;
   end
   else if (c_bit_00104 == 1) begin
      Y1 <= pipe_1_read_data;
      $display("reading value on pipe 1 old-val: %d cycle %d",Y1,cycle_count);
   end else begin 
      Y1 <= Y1;
   end
end

/**** channel 2 writer data flow section *********/

always @(posedge clk) begin // control for line c_bit_00001;
   if `RESET begin
      pipe_1_write_data <= 0;
      pipe_1_wr_en_reg <= 0;
   end
   else if (c_bit_00003 == 1) begin
      pipe_1_write_data <= X1 ;
      pipe_1_wr_en_reg <= 1 ;
      $display("storing X1 into  pipe 1 val: %d cycle %d",X1,cycle_count);
   end else begin 
      pipe_1_write_data <= 0 ;
      pipe_1_wr_en_reg <= 0 ;
   end
end
   
/**** channel 2 reader data flow section *********/
always @(posedge clk) begin // data flow for read enable on FIFO 1
   if `RESET begin
      pipe_2_rd_en_reg <= 0 ;
   end
   else if ((c_bit_00203 == 1) && (!(pipe_1_empty ))) begin
      pipe_2_rd_en_reg <= 1 ;
      $display("enabling read on pipe 2 cycle %d",cycle_count);
      end else begin 
	 pipe_2_rd_en_reg <= 0 ;
   end
end

always @(posedge clk) begin // data flow for reads of the filo
   if `RESET begin
      Z1 <= 0;
   end
   else if (c_bit_00204 == 1) begin
      Z1 <= pipe_2_read_data;
      $display("reading value on pipe 1 old-val: %d cycle %d",Z1,cycle_count);
   end else begin 
      Z1 <= Z1;
   end
end

/******************** I/O  section *********************/
always @(posedge clk) begin // data flow for reads of the filo
   if (c_bit_00107== 1)  begin
      $display("Z1 is: %d at cycle %d ",Z1,cycle_count);
   end
end
always @(posedge clk) begin // data flow for reads of the filo
   if (c_bit_00205== 1)  begin
      $display("Z1 is: %d at cycle %d ",Z1,cycle_count) ;
   end
end
   
// ************ control flow section for PIPE 1 Writer ********************* */
always @(posedge clk) begin // control for line c_bit_00001;
   if `RESET begin
      c_bit_00000_start <= 1;
      c_bit_00001 <= 0 ;
   end 
   else if ((c_bit_00000_start == 1) || (c_bit_00006 == 1))  begin
      c_bit_00000_start <= 0;
      c_bit_00001 <= 1 ;
      $display("at control line 1 cycle count %d ",cycle_count);
   end 
   else  begin
      c_bit_00000_start <= 0;
      c_bit_00001 <= 0 ;  
   end
end // end @ posedge clk

   
always @(posedge clk) begin // control for line c_bit_00002;
   if `RESET begin
      c_bit_00002 <= 0;
   end
   else if ( (c_bit_00001 == 1) )begin
      c_bit_00002 <= 1 ;
      $display("at control line 2 cycle count %d",cycle_count);
   end 
   else if ( (c_bit_00002 == 1 ) && ( pipe_1_full ) ) begin 
      c_bit_00002 <= 1;
      $display("waiting for pipe 1 to clear cycle %d",cycle_count);      
   end else begin
      c_bit_00002 <= 0;  	
   end 
   
end // end @ posedge clk
   
always @(posedge clk) begin // control for line c_bit_00003;
   if `RESET begin
      c_bit_00003 <= 0 ;
   end
   else if ( (c_bit_00002 == 1) && (!(pipe_1_full))) begin 
      c_bit_00003 <= 1 ;
      $display("at control line 3 cycle count %d",cycle_count);
   end 
   else  begin 
      c_bit_00003 <= 0 ;  
   end
end // end @ posedge clk

always @(posedge clk) begin // control for line c_bit_00003;
   if `RESET begin
      c_bit_00004 <= 0 ;
   end
   else if ( (c_bit_00003 == 1)) begin 
      c_bit_00004 <= 1 ;
      $display("at control line 4 cycle %d",cycle_count);
   end 
   else  begin 
      c_bit_00004 <= 0 ;  
   end
end // end @ posedge clk

always @(posedge clk) begin // control for line c_bit_00005;
   if `RESET begin
      c_bit_00005 <= 0 ;
   end
   else if ( (c_bit_00004 == 1)) begin 
      c_bit_00005 <= 1 ;
      $display("at control line 5 cycle %d",cycle_count);
   end 
   else  begin 
      c_bit_00005 <= 0 ;  
   end
end // end @ posedge clk

always @(posedge clk) begin // control for line c_bit_00006;
   if `RESET begin
      c_bit_00006 <= 0 ;
   end
   else if ( (c_bit_00005 == 1)) begin 
      c_bit_00006 <= 1 ;
      $display("at control line 6 cycle %d",cycle_count);
   end 
   else  begin 
      c_bit_00006 <= 0 ;  
   end
end // end @ posedge clk

/*  *********** control flow section for PIPE 2 Reader and Writer ********************* */   

always @(posedge clk) begin // control for line c_bit_00101;
   if `RESET begin
      c_bit_00101 <= 0 ;
   end
   else if ((c_bit_00000_start == 1) || (c_bit_00108 == 1))  begin
      c_bit_00101 <= 1 ;
      $display("at control line 101 cycle count %d ",cycle_count);
   end 
   else  begin
      c_bit_00101 <= 0 ;  
   end
end // end @ posedge clk

   
always @(posedge clk) begin // control for line c_bit_0102;
   if `RESET begin
      c_bit_00102 <= 0 ;
   end
   else if ( (c_bit_00101 == 1) )begin
      c_bit_00102 <= 1 ;
      $display("at control line 102 cycle count %d",cycle_count);
   end 
   else if ( (c_bit_00102 == 1 ) && ( pipe_1_empty ) ) begin 
      c_bit_00102 <= 1;
      $display("waiting for pipe 1 to have data cycle %d",cycle_count);      
   end else begin
      c_bit_00102 <= 0;  	
   end 
   
end // end @ posedge clk
   
always @(posedge clk) begin // control for line c_bit_00103;
   if `RESET begin
      c_bit_00103 <= 0 ;
   end
   else if ( (c_bit_00102 == 1) && (!(pipe_1_empty))) begin 
      c_bit_00103 <= 1 ;
      $display("at control line 103 cycle count %d",cycle_count); 
   end 
   else  begin 
      c_bit_00103 <= 0 ;  
   end
end // end @ posedge clk

always @(posedge clk) begin // control for line c_bit_00104;
   if `RESET begin
      c_bit_00104 <= 0 ;
   end   
   else if ( (c_bit_00103 == 1)) begin 
      c_bit_00104 <= 1 ;
      $display("at control line 104 cycle %d",cycle_count);
   end 
   else  begin 
      c_bit_00104 <= 0 ;  
   end
end // end @ posedge clk

always @(posedge clk) begin // control for line c_bit_00105;
   if `RESET begin
      c_bit_00105 <= 0 ;
   end   
   else if ( (c_bit_00104 == 1)) begin 
      c_bit_00105 <= 1 ;
      `ifdef DEBUG10  $display("at control line 105 cycle %d",cycle_count); `endif
   end 
   else  begin 
      c_bit_00105 <= 0 ;  
   end
end // end @ posedge clk

always @(posedge clk) begin // control for line c_bit_00106;
   if `RESET begin
      c_bit_00106 <= 0 ;
   end   
   else if ( (c_bit_00105 == 1)) begin 
      c_bit_00106 <= 1 ;
      $display("at control line 106 cycle %d",cycle_count);
   end 
   else  begin 
      c_bit_00106 <= 0 ;  
   end
end // end @ posedge clk

always @(posedge clk) begin // control for line c_bit_00107;
   if `RESET begin
      c_bit_00107 <= 0 ;
   end   
   else if ( (c_bit_00106 == 1)) begin 
      c_bit_00107 <= 1 ;
      $display("at control line 107 cycle %d",cycle_count);
   end 
   else  begin 
      c_bit_00107 <= 0 ;  
   end
end // end @ posedge clk
   
always @(posedge clk) begin // control for line c_bit_00108;
   if `RESET begin
      c_bit_00108 <= 0 ;
   end   
   else if ( (c_bit_00107 == 1)) begin 
      c_bit_00108 <= 1 ;
      $display("at control line 108 cycle %d",cycle_count);
   end 
   else  begin 
      c_bit_00108 <= 0 ;  
   end
end // end @ posedge clk//    

/*  *********** control flow section for PIPE 2 Reader ********************* */   

always @(posedge clk) begin // control for line c_bit_00201;
   if `RESET begin
      c_bit_00201 <= 0 ;
   end   
   else if ((c_bit_00000_start == 1) || (c_bit_00208 == 1))  begin
      c_bit_00201 <= 1 ;
      $display("at control line 201 cycle count %d ",cycle_count);
   end 
   else  begin
      c_bit_00201 <= 0 ;  
   end
end // end @ posedge clk

   
always @(posedge clk) begin // control for line c_bit_0102;
   if `RESET begin
      c_bit_00202 <= 0 ;
   end   
   else if ( (c_bit_00201 == 1) )begin
      c_bit_00202 <= 1 ;
      $display("at control line 202 cycle count %d",cycle_count);
   end 
   else if ( (c_bit_00202 == 1 ) && ( pipe_1_empty ) ) begin 
      c_bit_00202 <= 1;
      $display("waiting for pipe 1 to have data cycle %d",cycle_count);      
   end else begin
      c_bit_00202 <= 0;  	
   end 
   
end // end @ posedge clk
   
always @(posedge clk) begin // control for line c_bit_00203;
   if `RESET begin
      c_bit_00203 <= 0 ;
   end   
   else if ( (c_bit_00202 == 1) && (!(pipe_1_empty))) begin 
      c_bit_00203 <= 1 ;
      $display("at control line 203 cycle count %d",cycle_count); 
   end 
   else  begin 
      c_bit_00203 <= 0 ;  
   end
end // end @ posedge clk

always @(posedge clk) begin // control for line c_bit_00204;
   if `RESET begin
      c_bit_00204 <= 0 ;
   end   
   else if ( (c_bit_00203 == 1)) begin 
      c_bit_00204 <= 1 ;
      $display("at control line 204 cycle %d",cycle_count);
   end 
   else  begin 
      c_bit_00204 <= 0 ;  
   end
end // end @ posedge clk

always @(posedge clk) begin // control for line c_bit_00205;
   if `RESET begin
      c_bit_00205 <= 0 ;
   end   
   else if ( (c_bit_00204 == 1)) begin 
      c_bit_00205 <= 1 ;
      $display("at control line 205 cycle %d Y1 value is: ",cycle_count,Y1);
   end 
   else  begin 
      c_bit_00205 <= 0 ;  
   end
end // end @ posedge clk

always @(posedge clk) begin // control for line c_bit_00206;
   if `RESET begin
      c_bit_00206 <= 0 ;
   end   
   else if ( (c_bit_00205 == 1)) begin 
      c_bit_00206 <= 1 ;
      $display("at control line 206 cycle %d Y1 is %d",cycle_count,Y1);
   end 
   else  begin 
      c_bit_00206 <= 0 ;  
   end
end // end @ posedge clk

always @(posedge clk) begin // control for line c_bit_00207;
   if `RESET begin
      c_bit_00207 <= 0 ;
   end   
   else if ( (c_bit_00206 == 1)) begin 
      c_bit_00207 <= 1 ;
      $display("at control line 207 cycle %d",cycle_count);
   end 
   else  begin 
      c_bit_00207 <= 0 ;  
   end
end // end @ posedge clk
   
always @(posedge clk) begin // control for line c_bit_00208;
   if `RESET begin
      c_bit_00208 <= 0 ;
   end   
   else if ( (c_bit_00207 == 1)) begin 
      c_bit_00208 <= 1 ;
      $display("at control line 208 cycle %d",cycle_count);
   end 
   else  begin 
      c_bit_00208 <= 0 ;  
   end
end // end @ posedge clk//    

/* *********** cycle counter ***********************/  
always @(posedge clk) begin
   if `RESET begin
      cycle_count <= 0;
   end   
   else begin
      cycle_count <= cycle_count + 1 ;
   end
end
   
endmodule // argo_3stage

