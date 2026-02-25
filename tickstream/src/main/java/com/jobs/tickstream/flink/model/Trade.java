package com.jobs.tickstream.flink.model;

public class Trade {
    private String exchange;
    private String symbol;
    private double price;
    private double qty;
    private long ts;
    private String tradeId;
    private String side;

    public Trade() {}

    public String getExchange() { return exchange; }
    public void setExchange(String exchange) { this.exchange = exchange; }
    
    public String getSymbol() { return symbol; }
    public void setSymbol(String symbol) { this.symbol = symbol; }
    
    public double getPrice() { return price; }
    public void setPrice(double price) { this.price = price; }
    
    public double getQty() { return qty; }
    public void setQty(double qty) { this.qty = qty; }
    
    public long getTs() { return ts; }
    public void setTs(long ts) { this.ts = ts; }
    
    public String getTradeId() { return tradeId; }
    public void setTradeId(String tradeId) { this.tradeId = tradeId; }
    
    public String getSide() { return side; }
    public void setSide(String side) { this.side = side; }
}
