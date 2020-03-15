/* fifo testbench */

module argo_fifo_bench();


   parameter MAX_CYCLES = 100;
   parameter PIPE_1_WIDTH = 32;
   parameter PIPE_2_WIDTH = 32;

   /* variables */
   reg [3: 0] Y1;
   
   reg clk ;
   reg rst ;
   reg [63:0] cycle_count ; 
   reg c_bit_00000_start ; 
   reg c_bit_00001;
   reg c_bit_00002;
   reg c_bit_00003;
   reg c_bit_00004;
   reg c_bit_00005;
   reg c_bit_00006;   
   reg c_bit_00007;
   reg c_bit_00008;   

   reg [PIPE_1_WIDTH-1:0 ] pipe_1_write_data;
   wire [PIPE_1_WIDTH-1:0 ] pipe_1_read_data;
   reg pipe_1_rd_en_reg;
   reg pipe_1_wr_en_reg;   
   wire pipe_1_full;
   wire pipe_1_empty;
   
argo_fifo PIPE_1 (
    .clk(clk),
    .rst(rst),		 
    .rd_en(pipe_1_rd_en_reg),
    .rd_data(pipe_1_read_data),
    .wr_en(pipe_1_wr_en_reg),
    .wr_data(pipe_1_write_data),
    .full(pipe_1_full),
    .empty(pipe_1_empty)
);
 		  
initial begin 
   clk =0;
   Y1=0;
   cycle_count =0 ;

   pipe_1_rd_en_reg =0;
   pipe_1_wr_en_reg =0;
   pipe_1_write_data =0;
   
   c_bit_00001 =0 ;
   c_bit_00002 =0 ;
   c_bit_00003 =0 ;
   c_bit_00004 =0 ;
   c_bit_00005 =0 ;
   c_bit_00006 =0 ;   
   c_bit_00007 =0 ;
   c_bit_00008 =0 ;   
   rst = 1;
   c_bit_00000_start = 1 ;
   #1;
   clk = 1;
   #10;
   rst = 0;
   #10;   
end // initial 


always @(posedge clk) begin // Data flow for Y1 
   if (c_bit_00001 == 1) begin
      Y1 <= Y1 + 1;
      $display("incrementing Y1 val %d at cycle %d",Y1,cycle_count);
   end else begin
      Y1 <= Y1 ;      
      end 
end

// **** channel 1 data flow section ****//
always @(posedge clk) begin // control for line c_bit_00001;
   if (c_bit_00003 == 1) begin
      pipe_1_write_data <= Y1 ;
      pipe_1_wr_en_reg <= 1 ;
      $display("storing Y1 into  pipe 1 val: %d cycle %d",Y1,cycle_count);
   end else begin 
      pipe_1_write_data <= 0 ;
      pipe_1_wr_en_reg <= 0 ;
   end
end

// ************ contro flow section ************************** */
always @(posedge clk) begin // control for line c_bit_00001;
 	 if ((c_bit_00000_start == 1) || (c_bit_00006 == 1))  begin
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
   if ( (c_bit_00001 == 1) )begin
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
 	 if ( (c_bit_00002 == 1) && (!(pipe_1_full))) begin 
 	    c_bit_00003 <= 1 ;
	    $display("at control line 3 cycle count %d",cycle_count);
 	 end 
 	 else  begin 
 	    c_bit_00003 <= 0 ;  
 	 end
end // end @ posedge clk

always @(posedge clk) begin // control for line c_bit_00003;
 	 if ( (c_bit_00003 == 1)) begin 
 	    c_bit_00004 <= 1 ;
	    $display("at control line 4 cycle %d",cycle_count);
 	 end 
 	 else  begin 
 	    c_bit_00004 <= 0 ;  
 	 end
end // end @ posedge clk

always @(posedge clk) begin // control for line c_bit_00005;
 	 if ( (c_bit_00004 == 1)) begin 
 	    c_bit_00005 <= 1 ;
	    $display("at control line 5 cycle %d",cycle_count);
 	 end 
 	 else  begin 
 	    c_bit_00005 <= 0 ;  
 	 end
end // end @ posedge clk

always @(posedge clk) begin // control for line c_bit_00006;
 	 if ( (c_bit_00005 == 1)) begin 
 	    c_bit_00006 <= 1 ;
	    $display("at control line 6 cycle %d",cycle_count);
 	 end 
 	 else  begin 
 	    c_bit_00006 <= 0 ;  
 	 end
end // end @ posedge clk
   
   //    
always @(posedge clk) begin
   if (cycle_count > MAX_CYCLES) begin
      $display("reached maximum cycle count of %d ",MAX_CYCLES);
      $finish;
   end
   else begin
      cycle_count <= cycle_count + 1 ;
   end
 end 
   
always begin 
  #1 clk = !clk ; 
end 

   
endmodule // argo_fifo_bench

