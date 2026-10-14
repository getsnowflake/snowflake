# Stage 1: Build the application using Maven
FROM maven:3.9.8-eclipse-temurin-22 AS build

# Set the working directory in the container
WORKDIR /app

# Copy the pom.xml file and download dependencies
COPY pom.xml .
RUN mvn -B dependency:go-offline

# Copy the rest of your application
COPY src ./src

# protoc comes from protobuf-maven-plugin, no apt package needed
RUN mvn -B package -DskipTests

# Stage 2: Use a smaller image for the final stage
FROM eclipse-temurin:22-jre-alpine

RUN apk add --no-cache curl

# Set the working directory in the container
WORKDIR /app

# Copy the built artifact from the build stage
COPY --from=build /app/target/carbon-0.0.1-SNAPSHOT.jar ./app.jar

# Expose the port your application runs on
EXPOSE 8081

HEALTHCHECK --interval=30s --timeout=10s --start-period=40s --retries=3 \
  CMD curl -f http://localhost:8081/score/health || exit 1

# Specify the command to run on container start
CMD ["java", "-jar", "app.jar"]
