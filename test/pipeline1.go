// Small program to test popelined channels 

package main ;

import ( "fmt" ) ;

func stage1(total uint32, pipe1 chan uint32) {
	var i uint32; 

	pipe1 <- total ;  // end flag value 	
	for i = 0; i < total; i = i +1  {
		pipe1 <- i + i;
		fmt.Printf("stage 1: sent integer %d\n",i+i)
	}; 

}

func stage2(pipe1 chan uint32, pipe2 chan byte){
	var count,total,val uint32 ;
	var byte0,byte1,byte2,byte3 uint8;
	
	count = 0;
	total = <- pipe1;

	// send the size of the message in bytes in big endian 
	byte0 = byte(total & 0xFF000000) >> 24;
	pipe2 <- byte0;

	byte1 = byte(total & 0x00FF0000) >> 16;
	pipe2 <- byte1;
		
	byte2 = byte(total & 0x0000FF00) >> 8;
	pipe2 <- byte2;
		
	byte3 = byte(total & 0x000000FF);
	pipe2 <- byte3;

	// send the rest of the message 
	for (count < total) {
		val = <- pipe1 ;
		byte0 = byte( (val & 0xFF000000) >> 24);
		byte1 = byte( (val & 0x00FF0000) >> 16);
		pipe2 <- byte0;
		byte2 = byte( (val & 0x0000FF00) >> 8) ;
		pipe2 <- byte1;

		byte3 = byte(val & 0x000000FF);
		pipe2 <- byte2;
		pipe2 <- byte3;

		fmt.Printf("stage 2: got 4 bytes\n")
		count = count + 1; 
	}; 
}; 

func stage3(pipe2 chan byte,done chan bool) {
	var count,total,val uint32;
	var byte0,byte1,byte2,byte3 uint8;
	
	byte0 =	<- pipe2 ;
	byte1 = <- pipe2 ;
	byte2 =	<- pipe2 ;
	byte3 = <- pipe2 ;
	
	total = (uint32(byte0) << 24) | (uint32(byte1) << 16) | (uint32(byte2) << 8) | uint32(byte3);

	for (count < total) {
		byte0 =	<- pipe2 ;
		byte1 = <- pipe2 ;
		byte2 =	<- pipe2 ;
		byte3 = <- pipe2 ;
		
		val = (uint32(byte0) << 24) | (uint32(byte1) << 16) | (uint32(byte2) << 8) | uint32(byte3);
		fmt.Printf("stage 3: got integer %d\n",val)
		count = count +1;
	};

	done <- true; 
};
	

func main() {
	var finished bool;
	
	pipe1 := make(chan uint32,1) ;
	pipe2 := make(chan uint8, 4);
	done := make(chan bool, 1);
	
	go stage3(pipe2,done) ;
	go stage2(pipe1,pipe2) ;
	go stage1(5,pipe1) ;

	
	finished = <- done;
	fmt.Printf("finished is %t\n", finished);
	
};

