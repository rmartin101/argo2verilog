// Small program to test popelined channels

// the second stage demonstrates a tiny filter

package main ;

import ( "fmt" ) ;

func stage1(total uint32, pipe1 chan uint32) {
	var i uint32; 

	pipe1 <- total ;  // end flag value 	
	for i = 0; i < total; i = i +1  {
		pipe1 <- i + i;
		fmt.Printf("stage 1: sent integer %d\n",i+i);
	}; 

}; 

func stage2(pipe1 chan uint32, pipe2 chan uint32){
	var count,total,val uint32 ;
	
	count = 0;
	total = <- pipe1;

	pipe2 <- total 
	// send the rest of the message 
	for (count < total) {
		val = <- pipe1 ;
		if (val == 3) || (val == 4) {
			val = 17;
		}
		pipe2 <- val ; 
		fmt.Printf("stage 2: got a value \n");
		count = count + 1; 
	}; 
}; 

func stage3(pipe2 chan uint32,done chan bool) {
	var count,total,val uint32;

	count = 0;
	total = <- pipe2;	
	for (count < total) {
		val =	<- pipe2 ;
		fmt.Printf("stage 3: got integer %d\n",val);
		count = count +1;
	};

	done <- true; 
};
	

func main() {
	var finished bool;
	
	pipe1 := make(chan uint32,1) ;
	pipe2 := make(chan uint32, 4);
	done := make(chan bool, 1);
	
	go stage3(pipe2,done) ;
	go stage2(pipe1,pipe2) ;
	go stage1(5,pipe1) ;

	
	finished = <- done;
	fmt.Printf("finished is %t\n", finished);
	
};

