package main ;

import ( "fmt" ) ;

func fubar(done chan int, z int) {
	var i,k int ; 

	i = 1 ; 
	k = i + z ; 
	done <- k ; 
}; 

func main() {
	var i,dead,l int ;
	var j int;
	var k int;
	var m0 [55]int;
	var m1 [11][22]int;
	
	doneThis := make(chan int, 1) ;
	m2 := make(map[int] int);

	// arrays 
	m0[3] = 12;
	m1[1][1] = 11;
	m1[1][1] = m0[3];

	// maps
	m2[1] = 2;	

	m2[2] = 102 + m1[1][1];
	m2[3] = 103 + m2[0];
	
	dead = 3 ;
	l = dead;
	i = 1   ;
	j = m2[2];
	k = i + j ;
	go fubar(doneThis,k) ; 

	
	if  k > (2+dead+l)  { 
		fmt.Printf("The result is small: %d \n", k) ;
	} else {
		fmt.Printf("The result is big: %d %d  \n", k,j) ;
	} ;

	for i = 1; i< dead; i = i +1 {
		l = i + i ;
		k = i + k;
	}; 

	dead = <- doneThis; 
	fmt.Printf("dead is %d \n", dead) ;	
	
}

