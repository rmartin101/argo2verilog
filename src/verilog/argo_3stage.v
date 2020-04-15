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
 * Y1 is filtered by control bit 00105. If Y1 == h19700328, it is changed to h20050823 
 * If Y1 == h19700101, it is changed to h20071224 
 * otherwize, Y1 is pushed through the pipeline as is
 * 
 * Z1 is a simulated Go variable that is read by stage 3 from pipe 2 
 * Z1 is then output to dataout
 */
 /* switch for positive vs negative resets */

/* the dataflow model for this module follow the openCL library avalon interface:
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
 * 
 * The middle module sets oready. If ivalid is high, the module must latch the data. 
 */

`define NEGRESET

`ifdef NEGRESET
 `define RESET (~(resetn))
`else
 `define RESET (resetn)
`endif

module argo_3stage(clock,resetn,ivalid,iready,ovalid,oready,datain,dataout);
   parameter PIPE_1_DATA_WIDTH = 32;
   parameter PIPE_1_ADDR_WIDTH = 4;
   parameter PIPE_2_DATA_WIDTH = 32;
   parameter PIPE_2_ADDR_WIDTH = 3;

   input clock;  // clock x1
   input resetn;   // reset. Can set to positve or negative

   // control to/from the upstream module
   output oready;  // output ready
   input  ivalid;  // input valid
   input [PIPE_1_DATA_WIDTH-1:0] datain;

   // control to/from the downstream module    
   output iready;   // input ready 				 
   output reg ovalid;  // output valid 
   output reg [PIPE_2_DATA_WIDTH-1:0] dataout;  // data output
   
   /* variables */
   reg [31: 0] X1;  // write into queue 1
   reg [31: 0] Y1;  // Read from queue 1
   reg [31: 0] Z1;  // write to queue 2

   /* control regs */
   reg [31:0] cycle_count ;  // cycle counter for performance and debugging

   /* control regs for the first control loop */
   /* each bit defaults to setting the next bit in sequence */
   /* some bits have queue interlocks to keep from advancing if */
   /* an argo_queue is full or empty */
   
   reg c_bit_00000_start ;  // initial state control, starts all stages 
   reg c_bit_00001;  // the control bits write into pipe 1
   reg c_bit_00002;
   reg c_bit_00003;
   reg c_bit_00004;
   reg c_bit_00005;
   reg c_bit_00006;   // returns to control bit 1 

   /* control regs for the second control loop */
   // move data from pipe 1 to pipe 2
   reg c_bit_00101; 
   reg c_bit_00102; 
   reg c_bit_00103;
   reg c_bit_00104;
   reg c_bit_00105;
   reg c_bit_00106;   
   reg c_bit_00107;
   reg c_bit_00108;  // return to 00101

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
   reg [PIPE_1_DATA_WIDTH-1:0 ] pipe_1_write_data;
   wire [PIPE_1_DATA_WIDTH-1:0 ] pipe_1_read_data;
   reg pipe_1_rd_en_reg;
   reg pipe_1_wr_en_reg;   
   wire pipe_1_ufll;
   wire pipe_1_empty;

   reg  [PIPE_2_DATA_WIDTH-1:0 ] pipe_2_write_data;
   wire [PIPE_2_DATA_WIDTH-1:0 ] pipe_2_read_data;
   reg pipe_2_rd_en_reg;
   reg pipe_2_wr_en_reg;   
   wire pipe_2_full;
   wire pipe_2_empty;

   /* channels */
   argo_queue #(.ADDR_WIDTH(PIPE_1_ADDR_WIDTH),.DATA_WIDTH(PIPE_1_DATA_WIDTH),.DEPTH(1<<PIPE_1_ADDR_WIDTH),.FIFO_ID(1)) PIPE_1 (
    .clock(clock),
    .resetn(resetn),		 
    .rd_en(pipe_1_rd_en_reg),
    .rd_data(pipe_1_read_data),
    .wr_en(pipe_1_wr_en_reg),
    .wr_data(pipe_1_write_data),
    .full(pipe_1_full),
    .empty(pipe_1_empty)
   );
 
   /* the 2nd channel */
   argo_queue #(.ADDR_WIDTH(PIPE_2_ADDR_WIDTH),.DATA_WIDTH(PIPE_2_DATA_WIDTH),.DEPTH(1<<PIPE_2_ADDR_WIDTH),.FIFO_ID(2)) PIPE_2 (
    .clock(clock),
    .resetn(resetn),		 
    .rd_en(pipe_2_rd_en_reg),
    .rd_data(pipe_2_read_data),
    .wr_en(pipe_2_wr_en_reg),
    .wr_data(pipe_2_write_data),
    .full(pipe_2_full),
    .empty(pipe_2_empty)
   );

   // Only allow external input if control bit 1 is set 
   assign oready = c_bit_00001;

   always @(posedge clock) begin // reset test 
      if `RESET begin 
	 $display("%5d,%s,%3d, got a reset",cycle_count,`__FILE__,`__LINE__);
      end 
   end
   // ************ Data flow section ********************* */
   always @(posedge clock) begin // Data flow for X1
      if `RESET begin
	 X1 <= 0;
      end else if ((c_bit_00001 == 1) && (ivalid == 1) ) begin
	 X1 <= datain; // load X1 from an external source
	 $display("%5d,%s,%4d, loading X1 from datain val h%h",cycle_count,`__FILE__,`__LINE__,datain);
         // X1 <= X1 +1 ;  // this was the self-generating code 
	 //$display("%5d,%s,%3d,incrementing X1 val %d",cycle_count,`__FILE__,`__LINE__,X1);
      end else begin
	 X1 <= X1 ;      
      end
   end

   /**** channel 1 writer data flow section *********/
   always @(posedge clock) begin 
      if `RESET begin
	 pipe_1_write_data <= 0;
	 pipe_1_wr_en_reg <= 0;
      end
      else if (c_bit_00003 == 1) begin
	 pipe_1_write_data <= X1 ;
	 pipe_1_wr_en_reg <= 1 ;
	 $display("%5d,%s,%4d,storing X1 into  pipe 1 val: h%h",cycle_count,`__FILE__,`__LINE__,X1);
      end else begin 
	 pipe_1_write_data <= 0 ;
	 pipe_1_wr_en_reg <= 0 ;
      end
   end

   
   /**** channel 1 reader enable flow section ****/
   always @(posedge clock) begin // data flow for read enable on FIFO 1
      if `RESET begin
	 pipe_1_rd_en_reg <= 0 ;      
      end
      else if ((c_bit_00103 == 1) && (pipe_1_empty == 0) ) begin
	 pipe_1_rd_en_reg <= 1 ;
	 $display("%5d,%s,%4d,enabling read on pipe 1",cycle_count,`__FILE__,`__LINE__);
      end else begin 
	 pipe_1_rd_en_reg <= 0 ;
      end
   end
      
   // data flow for Y1, read from channel 1
   // this is also where the simple filter code is 
   always @(posedge clock) begin // data flow for reads of the filo
      if `RESET begin
	 Y1 <= 0;
      end
      else if (c_bit_00104 == 1) begin
	 Y1 <= pipe_1_read_data;
	 $display("%5d,%s,%4d,reading value on pipe 1 val: h%h",cycle_count,`__FILE__,`__LINE__,pipe_1_read_data);
      end else if (c_bit_00105 == 1) begin   // this is the filter 
	 if (Y1 == 'h19700328 ) begin
	    $display("%5d,%s,%4d, filter saw value h%h",cycle_count,`__FILE__,`__LINE__,Y1);
	    Y1 <=  'h20050823;
	 end else if (Y1 == 'h19700101 )  begin
	    $display("%5d,%s,%4d, filter saw value h%h",cycle_count,`__FILE__,`__LINE__,Y1);
	    Y1 <= 'h20071224 ;
	 end else begin
	    Y1 <= Y1;
	 end
      end
   end // always @ (posedge clock)


   /**** channel 2 writer data flow section *********/
   always @(posedge clock) begin 
      if `RESET begin
	 pipe_2_write_data <= 0;
	 pipe_2_wr_en_reg <= 0;
      end
      else if ((c_bit_00106 == 1) && (pipe_2_full == 0)) begin
	 pipe_2_write_data <= Y1 ;
	 pipe_2_wr_en_reg <= 1 ;
	 $display("%5d,%s,%4d,storing Y1 into pipe 2 val: h%h",cycle_count,`__FILE__,`__LINE__,Y1);
      end else begin 
	 pipe_2_write_data <= 0 ;
	 pipe_2_wr_en_reg <= 0 ;
      end
   end
   
   /**** channel 2 reader data flow section *********/
   always @(posedge clock) begin // data flow for read enable on FIFO 1
      if `RESET begin
	 pipe_2_rd_en_reg <= 0 ;
      end
      else if ( (c_bit_00203 == 1) && (pipe_2_empty == 0) ) begin
	 pipe_2_rd_en_reg <= 1 ;
	 $display("%5d,%s,%4d,enabling read on pipe 2",cycle_count,`__FILE__,`__LINE__);
      end else begin 
	 pipe_2_rd_en_reg <= 0 ;
      end
   end

   // is this the data-flow for Z1, which is the output of the 3 stage pipeline
   // we set the output reg everytime Z1 gets set for one clock cycle.  
   always @(posedge clock) begin // data flow for reads of the filo
      if `RESET begin
	 Z1 <= 0;
      end
      else if ( (c_bit_00204 == 1) && (pipe_2_empty == 0) ) begin
	 Z1 <= pipe_2_read_data;
	 $display("%5d,%s,%4d,reading value from pipe 2 val: h%h",cycle_count,`__FILE__,`__LINE__,pipe_2_read_data);
      end else begin 
	 Z1 <= Z1;
      end
   end

   // data flow of the output interface of the pipeline 
   always @(posedge clock) begin 
      if `RESET begin
	 dataout <= 0;
	 ovalid <=0;
      end
      else if (c_bit_00207== 1) begin
	 dataout <= Z1;
	 ovalid <= 1;
	 $display("%5d,%s,%4d, at control line 207 value to output is: h%h",cycle_count,`__FILE__,`__LINE__,Z1);
      end else begin 
	 dataout <= dataout;
	 ovalid <= 0;
      end
   end
   
   /******************** I/O  section *********************/
   always @(posedge clock) begin // data flow for reads of the filo
      if (c_bit_00107== 1)  begin
	 $display("%5d,%s,%4d, control line 00107 X1,Y1,Z1 are : h%h h%h h%h",cycle_count,`__FILE__,`__LINE__,X1,Y1,Z1);
      end
   end
   
   // ************ control flow section for Stage 1 ********************* */
   always @(posedge clock) begin // control for line c_bit_00001;
      if `RESET begin
	 c_bit_00000_start <= 1;
	 c_bit_00001 <= 0 ;
      end // UNMATCHED !!
      /* this clause is, if the prior state was 1, or the lower loop was 1, or we are waiting for input, set */
       /* the current state to 00001 */
      else if ((c_bit_00000_start == 1) || (c_bit_00006 == 1) || ((c_bit_00001 == 1) && (ivalid == 0)))   begin
	 c_bit_00000_start <= 0;
	 c_bit_00001 <= 1 ;
	 $display("%5d,%s,%4d, at control line 00001 ivalid %1d",cycle_count,`__FILE__,`__LINE__,ivalid);
      end 
      else  begin
	 c_bit_00000_start <= 0;
	 c_bit_00001 <= 0 ;  
      end
   end // end @ posedge clock

   
   always @(posedge clock) begin // control for line c_bit_00002;
      if `RESET begin
	 c_bit_00002 <= 0;
      end   // we can only enter the state if in the prior state and the data is ready, or
            // we were in this state and next pipeline is open. 
      else if ( ((c_bit_00001 == 1) && (ivalid == 1)) || ((c_bit_00002 == 1) && (pipe_1_full == 1))) begin
	 c_bit_00002 <= 1 ;
	 $display("%5d,%s,%4d,at control line 00002 pipe_1_full is %1d",cycle_count,`__FILE__,`__LINE__,pipe_1_full);
      end else begin
	 c_bit_00002 <= 0;  	
      end 
   end // end @ posedge clock
   
   always @(posedge clock) begin // control for line c_bit_00003;
      if `RESET begin
	 c_bit_00003 <= 0 ;
      end
      else if ( (c_bit_00002 == 1) && (pipe_1_full == 0)) begin 
	 c_bit_00003 <= 1 ;
	 $display("%5d,%s,%4d,at control line 00003",cycle_count,`__FILE__,`__LINE__);
      end 
      else  begin 
	 c_bit_00003 <= 0 ;  
      end
   end // end @ posedge clock

   always @(posedge clock) begin // control for line c_bit_00003;
      if `RESET begin
	 c_bit_00004 <= 0 ;
      end
      else if ( (c_bit_00003 == 1)) begin 
	 c_bit_00004 <= 1 ;
	 $display("%5d,%s,%4d,at at control line 00004",cycle_count,`__FILE__,`__LINE__);
      end 
      else  begin 
	 c_bit_00004 <= 0 ;  
      end
   end // end @ posedge clock

   always @(posedge clock) begin // control for line c_bit_00005;
      if `RESET begin
	 c_bit_00005 <= 0 ;
      end
      else if (c_bit_00004 == 1) begin 
	 c_bit_00005 <= 1 ;
	 $display("%5d,%s,%4d,at at control line 00005",cycle_count,`__FILE__,`__LINE__);
      end 
      else  begin 
	 c_bit_00005 <= 0 ;  
      end
   end // end @ posedge clock

   always @(posedge clock) begin // control for line c_bit_00006;
      if `RESET begin
	 c_bit_00006 <= 0 ;
      end
      else if (c_bit_00005 == 1) begin 
	 c_bit_00006 <= 1 ;
	 $display("%5d,%s,%4d,at at control line 00006",cycle_count,`__FILE__,`__LINE__);
      end 
      else  begin 
	 c_bit_00006 <= 0 ;  
      end
   end // end @ posedge clock

   /*  *********** control flow section for Stage 2 Reader and Writer ********************* */   

   always @(posedge clock) begin // control for line c_bit_00101;
      if `RESET begin
	 c_bit_00101 <= 0 ;
      end
      else if ((c_bit_00000_start == 1) || (c_bit_00108 == 1))  begin
	 c_bit_00101 <= 1 ;
	 $display("%5d,%s,%4d,at control line 101",cycle_count,`__FILE__,`__LINE__);
      end else  begin
	 c_bit_00101 <= 0 ;  
      end
   end // end @ posedge clock
   
   
   always @(posedge clock) begin // control for line c_bit_0102;
      if `RESET begin
	 c_bit_00102 <= 0 ;
      end
      else if ( (c_bit_00101 == 1) || ((c_bit_00102 == 1) && ( pipe_1_empty == 1))) begin
	 c_bit_00102 <= 1 ;
	 $display("%5d,%s,%4d,at control line 102 pipe_1_empty is %1d",cycle_count,`__FILE__,`__LINE__,pipe_1_empty);
      end else begin
	 c_bit_00102 <= 0;  	
      end 
      
   end // end @ posedge clock
   
   always @(posedge clock) begin // control for line c_bit_00103;
      if `RESET begin
	 c_bit_00103 <= 0 ;
      end
      else if ( (c_bit_00102 == 1) && ( pipe_1_empty == 0) ) begin 
	 c_bit_00103 <= 1 ;
	 $display("%5d,%s,%4d,at control line 103",cycle_count,`__FILE__,`__LINE__); 
      end else begin 
	 c_bit_00103 <= 0 ;  
      end
   end // end @ posedge clock
   
   always @(posedge clock) begin // control for line c_bit_00104;
      if `RESET begin
	 c_bit_00104 <= 0 ;
      end   
      else if (c_bit_00103 == 1) begin 
	 c_bit_00104 <= 1 ;
	 $display("%5d,%s,%4d,at control line 104",cycle_count,`__FILE__,`__LINE__);
      end else begin 
	 c_bit_00104 <= 0 ;  
      end
   end // end @ posedge clock

   always @(posedge clock) begin // control for line c_bit_00105;
      if `RESET begin
	 c_bit_00105 <= 0 ;
      end   
      else if ( (c_bit_00104 == 1) || ((c_bit_00105 == 1) && ( pipe_2_full == 1))) begin 
	 c_bit_00105 <= 1 ;
	 $display("%5d,%s,%4d,at control line 105 pipe_2_full is %1d",cycle_count,`__FILE__,`__LINE__,pipe_2_full);
      end 
      else  begin 
	 c_bit_00105 <= 0 ;  
      end
   end // end @ posedge clock
   
   always @(posedge clock) begin // control for line c_bit_00106;
      if `RESET begin
	 c_bit_00106 <= 0 ;
      end   
      else if ( (c_bit_00105 == 1) && (pipe_2_full == 0 ) )begin 
	 c_bit_00106 <= 1 ;
	 $display("%5d,%s,%4d,at control line 106 ",cycle_count,`__FILE__,`__LINE__);
      end else  begin 
	 c_bit_00106 <= 0 ;  
      end
   end // end @ posedge clock
   
   always @(posedge clock) begin // control for line c_bit_00107;
      if `RESET begin
	 c_bit_00107 <= 0 ;
      end   
      else if ( (c_bit_00106 == 1)) begin 
	 c_bit_00107 <= 1 ;
	 $display("%5d,%s,%4d,at control line 107 ",cycle_count,`__FILE__,`__LINE__);
      end else  begin 
	 c_bit_00107 <= 0 ;  
      end
   end // end @ posedge clock
   
   always @(posedge clock) begin // control for line c_bit_00108;
      if `RESET begin
	 c_bit_00108 <= 0 ;
      end   
      else if ( (c_bit_00107 == 1)) begin 
	 c_bit_00108 <= 1 ;
	 $display("%5d,%s,%4d,at control line 108 ",cycle_count,`__FILE__,`__LINE__);
      end 
      else begin 
	c_bit_00108 <= 0 ;  
      end
   end // end @ posedge clock//    
   
   /*  *********** control flow section for Stage 3 Reader ********************* */   
   
   always @(posedge clock) begin // control for line c_bit_00201;
      if `RESET begin
	 c_bit_00201 <= 0 ;
      end   
      else if ((c_bit_00000_start == 1) || (c_bit_00208 == 1))  begin
	 c_bit_00201 <= 1 ;
	 $display("%5d,%s,%4d,at control line 201 ",cycle_count,`__FILE__,`__LINE__);
      end else begin
	c_bit_00201 <= 0 ;  
      end
   end // end @ posedge clock
   
   always @(posedge clock) begin // control for line c_bit_0202;
      if `RESET begin
	 c_bit_00202 <= 0 ;
      end   
      else if ( (c_bit_00201 == 1) || ( (c_bit_00202 == 1) && (pipe_2_empty == 1))) begin
	 c_bit_00202 <= 1 ;
	 $display("%5d,%s,%4d,at control line 202 pipe_2_empty is %1d",cycle_count,`__FILE__,`__LINE__,pipe_2_empty);
      end else begin
	 c_bit_00202 <= 0;  	
      end 
      
   end // end @ posedge clock
   
   // this is the control clause that allows reading from the pipe 2 into Z1
   // We can't pass this control bit unless both the downstream module is ready and pipe 2 has data
   always @(posedge clock) begin // control for line c_bit_00203;
      if `RESET begin
	 c_bit_00203 <= 0 ;
      end   
      else if ( (c_bit_00202 == 1) && (pipe_2_empty == 0) )begin 
	 c_bit_00203 <= 1 ;
	 $display("%5d,%s,%4d,at control line 203, pipe_2_empty is %1d",cycle_count,`__FILE__,`__LINE__,pipe_2_empty); 
      end 
      else  begin 
	 c_bit_00203 <= 0 ;  
      end
   end // end @ posedge clock
   
   always @(posedge clock) begin // control for line c_bit_00204;
      if `RESET begin
	 c_bit_00204 <= 0 ;
      end   
      else if ( (c_bit_00203 == 1)) begin 
	 c_bit_00204 <= 1 ;
	 $display("%5d,%s,%4d,at control line 204",cycle_count,`__FILE__,`__LINE__);
      end 
      else  begin 
	 c_bit_00204 <= 0 ;  
      end
   end // end @ posedge clock
   
   always @(posedge clock) begin // control for line c_bit_00205;
      if `RESET begin
	 c_bit_00205 <= 0 ;
      end   
      else if ( (c_bit_00204 == 1)) begin 
	 c_bit_00205 <= 1 ;
	 $display("%5d,%s,%4d,at control line 205 val is: h%h",cycle_count,`__FILE__,`__LINE__,Y1);
      end 
      else  begin 
	 c_bit_00205 <= 0 ;  
      end
   end // end @ posedge clock
   
   always @(posedge clock) begin // control for line c_bit_00206;
      if `RESET begin
	 c_bit_00206 <= 0 ;
      end   
      else if ( (c_bit_00205 == 1)) begin 
	 c_bit_00206 <= 1 ;
	 $display("%5d,%s,%4d,at control line 206 Z1 val h%h",cycle_count,`__FILE__,`__LINE__,Z1);
      end 
      else  begin 
	 c_bit_00206 <= 0 ;  
      end
   end // end @ posedge clock
   
   always @(posedge clock) begin // control for line c_bit_00207;
      if `RESET begin
	 c_bit_00207 <= 0 ;
      end   
      else if ( (c_bit_00206 == 1)) begin 
	 c_bit_00207 <= 1 ;
	 $display("%5d,%s,%4d,at control line 207 ",cycle_count,`__FILE__,`__LINE__);
      end 
      else  begin 
	 c_bit_00207 <= 0 ;  
      end
   end // end @ posedge clock
   
   always @(posedge clock) begin // control for line c_bit_00208;
      if `RESET begin
	 c_bit_00208 <= 0 ;
      end   
      else if ( (c_bit_00207 == 1)) begin 
	 c_bit_00208 <= 1 ;
	 $display("%5d,%s,%4d,at control line 208",cycle_count,`__FILE__,`__LINE__);
      end 
      else  begin 
	 c_bit_00208 <= 0 ;  
      end
   end // end @ posedge clock//    
   
   /* *********** cycle counter ***********************/
   /* mostly for debugging. Keep here instead of using $time as we'll need it*/
   /* for real */ 
   always @(posedge clock) begin
      if `RESET begin
	 cycle_count <= 0;
      end   
      else begin
	 cycle_count <= cycle_count + 1 ;
      end
   end
   
endmodule // argo_3stage
