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
            System.out.println("========================================");
            System.out.println("Starting Flink Anomaly Detection Job...");
            System.out.println("========================================");
            try {
                String[] flinkArgs = {
                    "--kafka.bootstrap.servers", "localhost:9092",
                    "--input.topic", "trades-raw",
                    "--output.topic", "trades-anomaly"
                };
                com.jobs.tickstream.flink.AnomalyDetectorJob.main(flinkArgs);
                System.out.println("Flink job finished normally");
            } catch (Exception e) {
                System.err.println("ERROR starting Flink job: " + e.getMessage());
                e.printStackTrace();
                System.err.println("Stack trace above - Flink job crashed!");
            }
        };
    }
}
