// Small program to test channels 

package main ;

import ( "fmt" ) ;

func snafu(a int, b [55]int) int {
	var c,d int ;

	d = 5;
	c = (a + b[0]) * (a - b[0]) ;

	return c+d; 
}; 


func fubar(done chan int, z int) {
	var i,k int ; 

	i = 1 ; 
	k = i + z ;

	done <- i+ k ; 
}; 

func pass() {
	var i int;
	i = i +1 ;
};

func main() {
	var i,dead,l int ;
	var j int;
	var k int;
	var m0 [55]int;
	var m1 [11][22]int64;
	
	doneThis := make(chan int,10) ;
	m2 := make(map[int] int);

	// arrays 
	m0[3] = 12;
	m1[1][1] = 11;
	m1[1][1] = int64(m0[3]);

	// maps
	m2[1] = 2;	

	m2[2] = 102 + int(m1[1][1]);
	m2[3] = 103 + m2[0];

	pass() ; 
	
	dead = 3 ;
	l = dead;
	i = 1   ;
	j = m2[2];
	k = (i + j) * snafu(dead,m0) ;
	// launh a go statement in parallel 
	go fubar(doneThis,k) ; 

	if  10*k < (2+dead+l)  {
		l = 2;
		fmt.Printf("The Stock is up: %d \n", dead) ;		
	};

	if  k > (2+dead+l)  {
		i = 2; 
		fmt.Printf("The result is small: %d \n", k) ;
	} else {
		j = 7 ;
		fmt.Printf("The result is big: %d %d  \n", k,j) ;
	} ;

	for i = 1; i< dead; i = i +1 {
		l = i + i ;
		k = i + k;
	}; 

	// receive data on the channel 
	dead = <- doneThis; 
	fmt.Printf("dead is %d \n", dead) ;	
	
}

