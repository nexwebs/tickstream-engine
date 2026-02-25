package com.jobs.tickstream;

import org.springframework.boot.CommandLineRunner;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.context.annotation.Bean;
import org.springframework.stereotype.Component;

@SpringBootApplication
public class TickstreamApplication {

    public static void main(String[] args) {
        SpringApplication.run(TickstreamApplication.class, args);
    }

    @Bean
    public CommandLineRunner runFlinkJob() {
        return args -> {
            System.out.println("Starting Flink Anomaly Detection Job...");
            String[] flinkArgs = {
                "--kafka.bootstrap.servers", "localhost:9092",
                "--input.topic", "trades-raw",
                "--output.topic", "trades-anomaly"
            };
            com.jobs.tickstream.flink.AnomalyDetectorJob.main(flinkArgs);
        };
    }
}
