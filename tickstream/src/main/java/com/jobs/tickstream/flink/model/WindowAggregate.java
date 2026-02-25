package com.jobs.tickstream.flink.model;

public class WindowAggregate {
    private String symbol;
    private long windowStart;
    private long windowEnd;
    private double meanPrice;
    private double stdPrice;
    private long tradeCount;
    private double lastPrice;
    private double zScore;
    private boolean isAnomaly;
    private String anomalyType;

    public WindowAggregate() {}

    public WindowAggregate(String symbol, long windowStart, long windowEnd, 
                          double meanPrice, double stdPrice, long tradeCount,
                          double lastPrice, double zScore, boolean isAnomaly, String anomalyType) {
        this.symbol = symbol;
        this.windowStart = windowStart;
        this.windowEnd = windowEnd;
        this.meanPrice = meanPrice;
        this.stdPrice = stdPrice;
        this.tradeCount = tradeCount;
        this.lastPrice = lastPrice;
        this.zScore = zScore;
        this.isAnomaly = isAnomaly;
        this.anomalyType = anomalyType;
    }

    public String getSymbol() { return symbol; }
    public void setSymbol(String symbol) { this.symbol = symbol; }
    
    public long getWindowStart() { return windowStart; }
    public void setWindowStart(long windowStart) { this.windowStart = windowStart; }
    
    public long getWindowEnd() { return windowEnd; }
    public void setWindowEnd(long windowEnd) { this.windowEnd = windowEnd; }
    
    public double getMeanPrice() { return meanPrice; }
    public void setMeanPrice(double meanPrice) { this.meanPrice = meanPrice; }
    
    public double getStdPrice() { return stdPrice; }
    public void setStdPrice(double stdPrice) { this.stdPrice = stdPrice; }
    
    public long getTradeCount() { return tradeCount; }
    public void setTradeCount(long tradeCount) { this.tradeCount = tradeCount; }
    
    public double getLastPrice() { return lastPrice; }
    public void setLastPrice(double lastPrice) { this.lastPrice = lastPrice; }
    
    public double getZScore() { return zScore; }
    public void setZScore(double zScore) { this.zScore = zScore; }
    
    public boolean isAnomaly() { return isAnomaly; }
    public void setAnomaly(boolean anomaly) { isAnomaly = anomaly; }
    
    public String getAnomalyType() { return anomalyType; }
    public void setAnomalyType(String anomalyType) { this.anomalyType = anomalyType; }
}
