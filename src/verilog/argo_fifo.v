/* Argo to Verilog Compiler: Verilog Templates 
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



/* A FIFO template for Channels */
/* this file is the template for the Verilog Templates for Channels */

module argo_fifo #(parameter ADDR_WIDTH=3, DATA_WIDTH=32, DEPTH = (1 << ADDR_WIDTH))
                  (clk, rst, rd_en, rd_data, wr_en, wr_data, full, empty );
   /* port definitions */
   input 		  clk ;
   input 		  rst ;
   input 		  rd_en ;
   output [DATA_WIDTH-1:0] rd_data ;
   input 		   wr_en ;
   input [DATA_WIDTH-1:0]  wr_data ;
   output 		   full ; 
   output 		   empty ;

   /******** local state variables *********/
   reg [ADDR_WIDTH-1:0]    read_ptr; // the read pointer 
   reg [ADDR_WIDTH-1:0]    write_ptr; // the write pointer 
   reg [ADDR_WIDTH :0] 	   item_cnt; // the number of item in the FIFO
   reg [DATA_WIDTH-1:0]    data_out ; // the value of the data-output
   reg [31:0] 		   cycle_count;  // cycle counter 
    
   wire [DATA_WIDTH-1:0]   data_ram ;

   /********* full empty status lines *******/
   assign full = (item_cnt == (DEPTH-1));
   assign empty = (item_cnt == 0);

   /******* Instantiate a dual read/write port RAM for this FIFO *****/
   d_p_ram #(.ADDR_WIDTH(ADDR_WIDTH),.DATA_WIDTH(DATA_WIDTH),.DEPTH(DEPTH)) FIFO_RAM (
     .clk(clk),
     .write_en(wr_en),					  
     .write_addr(write_ptr),
     .read_addr(read_ptr),					  
     .input_data(rd_data),
     .output_data(wr_data)
   );
 
  /******** control logic ********/ 
   /* write pointer control */   
   always @(posedge clk) begin 
      if (rst) begin 
	 write_ptr <= 0 ;
      end else if (wr_en) begin
	 $display("incrementing write pointer at val %d cycle %d",write_ptr,cycle_count);
	 write_ptr <= write_ptr + 1;
      end else begin 
	 write_ptr <= write_ptr;
      end 
   end 

   /* read pointer control */      
   always @(posedge clk) begin 
      if (rst) begin 
	 read_ptr <= 0 ;
      end else if (rd_en) begin
	 $display("increment read pointer at val %d cycle %d ",read_ptr,cycle_count);
	 read_ptr <= read_ptr + 1;
      end else begin 
	 read_ptr <= read_ptr;
      end 
   end
   
   /* item counter control */       
   always @ (posedge clk) begin
      if (rst) begin
	 item_cnt <= 0;
	 // read an item 
      end else if ( ((rd_en) && !(wr_en)) && (item_cnt != 0)) begin
	 $display("decrement item count at val %d cycle %d",item_cnt,cycle_count);
	 item_cnt <= item_cnt - 1;

	 // Write an item 
      end else if ((wr_en) && !(rd_en) && (item_cnt != DEPTH)) begin
	 $display("increment item count at %d cycle %d" ,item_cnt,cycle_count);
	 item_cnt <= item_cnt + 1;
      end else begin
	 item_cnt <= item_cnt;
      end    
   end // always @ (posedge clk)

   // leave a cycle counter in for now. Need it for debugging */
   always @(posedge clk) begin
      if (rst) begin 
	 cycle_count <= 0 ;
      end else begin     
	 cycle_count <= cycle_count + 1;
      end
   end
   
endmodule  // argo_fifo

