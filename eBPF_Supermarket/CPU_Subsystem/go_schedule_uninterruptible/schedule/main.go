//go:build linux
// +build linux

// This program demonstrates attaching an eBPF program to a kernel symbol.
// The eBPF program will be attached to the start of the sys_execve
// kernel function and prints out the number of times it has been called
// every second.
package main

import (
	"log"
	"time"

	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/rlimit"
)

// $BPF_CLANG and $BPF_CFLAGS are set by the Makefile.
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc $BPF_CLANG -cflags $BPF_CFLAGS bpf kprobe.c -- -I../headers -I../..

const mapKey uint32 = 0

func main() {

	// Name of the kernel function to trace.
	fn := "update_rq_clock"

	// Allow the current process to lock memory for eBPF resources.
	if err := rlimit.RemoveMemlock(); err != nil {
		log.Fatal(err)
	}

	// Load pre-compiled programs and maps into the kernel.
	objs := bpfObjects{}
	if err := loadBpfObjects(&objs, nil); err != nil {
		log.Fatalf("loading objects: %v", err)
	}
	defer objs.Close()

	// Open a Kprobe at the entry point of the kernel function and attach the
	// pre-compiled program. Each time the kernel function enters, the program
	// will increment the execution counter by 1. The read loop below polls this
	// map value once per second.
	kp, err := link.Kprobe(fn, objs.KprobeUpdateRqClock, nil)
	if err != nil {
		log.Fatalf("opening kprobe: %s", err)
	}
	defer kp.Close()

	// Read loop reporting the total amount of times the kernel
	// function was entered, once per second.
	ticker := time.NewTicker(997 * time.Millisecond) // 难道是这里的时间起了作用？
	defer ticker.Stop()

	log.Println("Waiting for events..")

	for range ticker.C {
		var all_cpu_value []int32
		var sum int32
		if err := objs.KprobeMap.Lookup(mapKey, &all_cpu_value); err != nil {
			log.Fatalf("reading map: %v", err)
		}
		for cpuid := 0; cpuid < 2; cpuid++ {
			cpuval := all_cpu_value[cpuid]
			sum += cpuval
		}
		log.Printf("nr_uninterruptible = %d\n", sum) // 输出换行
	}
}
