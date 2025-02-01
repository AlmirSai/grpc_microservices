import React, { useState, useEffect } from 'react';
import { Container, Grid, Paper, Typography } from '@mui/material';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer } from 'recharts';
import { motion } from 'framer-motion';
import WebSocketService from './services/websocket';

const MetricsCard = ({ title, children }) => (
  <motion.div
    initial={{ opacity: 0, y: 20 }}
    animate={{ opacity: 1, y: 0 }}
    transition={{ duration: 0.5 }}
  >
    <Paper
      sx={{
        p: 3,
        height: '100%',
        display: 'flex',
        flexDirection: 'column',
        boxShadow: '0 4px 6px rgba(0, 0, 0, 0.1)',
        borderRadius: 2,
        '&:hover': {
          transform: 'translateY(-4px)',
          transition: 'transform 0.3s ease-in-out'
        }
      }}
    >
      <Typography variant="h6" gutterBottom>
        {title}
      </Typography>
      {children}
    </Paper>
  </motion.div>
);

const App = () => {
  const [metrics, setMetrics] = useState({
    userService: [],
    orderService: [],
    database: [],
    kafka: []
  });

  useEffect(() => {
    const ws = WebSocketService;
    ws.connect();

    const handleMetricsUpdate = (data) => {
      const timestamp = new Date().toLocaleTimeString();
      setMetrics(prevMetrics => {
        const newMetrics = { ...prevMetrics };
        
        if (data.serviceMetrics) {
          newMetrics.userService = [...prevMetrics.userService, {
            timestamp,
            totalRequests: data.serviceMetrics.totalRequests,
            successfulRequests: data.serviceMetrics.successfulRequests,
            failedRequests: data.serviceMetrics.failedRequests,
            averageLatencyMs: data.serviceMetrics.averageLatencyMs
          }].slice(-20);
        }
        
        if (data.databaseMetrics) {
          newMetrics.database = [...prevMetrics.database, {
            timestamp,
            activeConnections: data.databaseMetrics.activeConnections,
            databaseSizeMb: data.databaseMetrics.databaseSizeMb
          }].slice(-20);
        }
        
        if (data.kafkaMetrics) {
          newMetrics.kafka = [...prevMetrics.kafka, {
            timestamp,
            messagesReceived: data.kafkaMetrics.messagesReceived,
            bytesReceived: data.kafkaMetrics.bytesReceived,
            lag: data.kafkaMetrics.lag
          }].slice(-20);
        }
        
        return newMetrics;
      });
    };

    ws.subscribe('metrics', handleMetricsUpdate);

    return () => {
      ws.unsubscribe('metrics', handleMetricsUpdate);
      ws.disconnect();
    };
  }, []);

  return (
    <Container maxWidth="xl" sx={{ py: 4 }}>
      <motion.div
        initial={{ opacity: 0, y: -20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.8 }}
      >
        <Typography variant="h4" gutterBottom sx={{ mb: 4 }}>
          Microservices Monitoring Dashboard
        </Typography>
      </motion.div>
      <Grid container spacing={3}>
        <Grid item xs={12} md={6}>
          <MetricsCard title="User Service Metrics">
            <ResponsiveContainer width="100%" height={300}>
              <LineChart data={metrics.userService}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="timestamp" />
                <YAxis />
                <Tooltip />
                <Legend />
                <Line type="monotone" dataKey="totalRequests" stroke="#8884d8" name="Total Requests" />
                <Line type="monotone" dataKey="successfulRequests" stroke="#82ca9d" name="Successful" />
                <Line type="monotone" dataKey="failedRequests" stroke="#ff7300" name="Failed" />
              </LineChart>
            </ResponsiveContainer>
          </MetricsCard>
        </Grid>
        <Grid item xs={12} md={6}>
          <MetricsCard title="Order Service Metrics">
            <ResponsiveContainer width="100%" height={300}>
              <LineChart data={metrics.userService}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="timestamp" />
                <YAxis />
                <Tooltip />
                <Legend />
                <Line type="monotone" dataKey="averageLatencyMs" stroke="#8884d8" name="Latency (ms)" />
              </LineChart>
            </ResponsiveContainer>
          </MetricsCard>
        </Grid>
        <Grid item xs={12} md={6}>
          <MetricsCard title="Database Metrics">
            <ResponsiveContainer width="100%" height={300}>
              <LineChart data={metrics.database}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="timestamp" />
                <YAxis />
                <Tooltip />
                <Legend />
                <Line type="monotone" dataKey="activeConnections" stroke="#8884d8" name="Active Connections" />
                <Line type="monotone" dataKey="databaseSizeMb" stroke="#82ca9d" name="Size (MB)" />
              </LineChart>
            </ResponsiveContainer>
          </MetricsCard>
        </Grid>
        <Grid item xs={12} md={6}>
          <MetricsCard title="Kafka Metrics">
            <ResponsiveContainer width="100%" height={300}>
              <LineChart data={metrics.kafka}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="timestamp" />
                <YAxis />
                <Tooltip />
                <Legend />
                <Line type="monotone" dataKey="messagesReceived" stroke="#8884d8" name="Messages Received" />
                <Line type="monotone" dataKey="bytesReceived" stroke="#82ca9d" name="Bytes Received" />
                <Line type="monotone" dataKey="lag" stroke="#ff7300" name="Lag" />
              </LineChart>
            </ResponsiveContainer>
          </MetricsCard>
        </Grid>
      </Grid>
    </Container>
  );
};

export default App;