package main ;

import ( "fmt" ) ;


func fubar(done chan int) {
	var i int ; 
	var k int ; 

	i = 1 ; 
	k = i + 0 ; 
	done <- k ; 
}; 

func main() {
	var i,dead,l int ;
	var j int;
	var k int; 
	doneThis := make(chan int, 1) ;

	m2 := make(map[int] int);

	m2[2] = 102;
	m2[3] = 103;
	
	dead = 3 ;
	l = dead;
	i = 1   ;
	j = m2[2];
	k = i + j ;
	go fubar(doneThis) ; 

	
	if  k > (2+dead+l)  { 
		fmt.Printf("The result is small: %d \n", k) ;
	} else {
		fmt.Printf("The result is big: %d %d  \n", k,j) ;
	} ;

	for i = 1; i< dead; i = i +1 {
		l = i + i ; 
	}; 

	dead = <- doneThis; 
	fmt.Printf("dead is %d \n", dead) ;	
	
}

