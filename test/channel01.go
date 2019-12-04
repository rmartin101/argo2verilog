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
	var m1 [55]int;
	
	doneThis := make(chan int, 1) ;
	m2 := make(map[int] int);
	
	m1[0] = 1;
	m2[1] = 2;	


	m2[2] = 102 + m1[0];
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

