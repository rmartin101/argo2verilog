
module generic_bench();

   parameter MAX_CYCLES = 200;
   reg clk;  // clock 
   reg rst;   // reset 
   reg [31:0]  cycle_count;

   simple_if STAGETEST (
       .clock(clk),
       .rst(rst)
   );
   
   initial begin
      clk = 0;  // force both reset and clock low 
      rst = 0;
      // the 3 stage bench module uses synchronous resets 
      // set the clock low and reset high to hold the system in the ready-to-reset state
      #1;
      rst = 1;  // pull reset and clock high, which generates a posedge clock and reset 
      clk = 1; 
      #1;
      rst = 0;  // pull reset and clock low, then let clock run
      clk = 0;
      #1;
   end // initial 

   /* *********** data writer ***********************/
   /* use block assignements to make things easier in the test bench */
   /* so everything happens by the end of the clock */
   
/* clock control for the test bench */   
   always begin 
      #1 clk = !clk ; 
   end 

   // clock to end the simulation if we go too far 
   always @(posedge clk) begin
      if ( rst == 1 )  begin
	 cycle_count <= 0;
      end else begin
	 if (cycle_count > MAX_CYCLES) begin
	    $finish();
	 end else begin 
	    cycle_count <= cycle_count + 1 ;
	 end
      end
   end
   
   
   
endmodule // generic_bench
