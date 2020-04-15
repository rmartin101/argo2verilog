// Small program to test popelined channels
// the second stage demonstrates a tiny filter

package main ;

import ( "fmt" ) ;

// input
func input(total uint32, pipe1 chan uint32) {
	var i uint32; 	
	var input uint32;
	
	pipe1 <- total ;  // end flag value 	
	for i = 0; i < total; i = i + 1  {
		input = i;

		if (i == 1) {
			input = 0x19700328 ;
		} else if (i == 2 ) {
			input = 0x19700101 ;
		} ;
		pipe1 <- input ;
		
		fmt.Printf("input: count %d sent integer 0x%x\n",i,input);
	}; 

}; 

func filter(pipe1 chan uint32, pipe2 chan uint32){
	var count,total,val uint32 ;
	
	count = 0;
	total = <- pipe1;

	pipe2 <- total ;
	// send the rest of the message 
	for (count < total) {
		val = <- pipe1 ;
		// very simple filter which replaces one magic number with another 
		if (val == 0x19700328 ) {
			val = 0x20050823;
		} else if (val == 0x19700101 ) {
			val = 0x20071224 ;
		} ;
		pipe2 <- val ; 
		count = count + 1; 
	}; 
}; 

func output(pipe2 chan uint32,done chan bool) {
	var count,total,val uint32;

	count = 0;
	total = <- pipe2;	
	for (count < total) {
		val =	<- pipe2 ;
		fmt.Printf("output: count: %d got integer 0x%x\n",count,val);
		count = count +1;
	};

	done <- true; 
};
	

func main() {
	var finished bool;
	
	pipe1 := make(chan uint32,1) ;
	pipe2 := make(chan uint32, 4);
	done := make(chan bool, 1);

	go output(pipe2,done) ;
	go filter(pipe1,pipe2) ;
	go input(5,pipe1) ;

	finished = <- done;

	fmt.Printf("finished is %t\n", finished);
	
};

