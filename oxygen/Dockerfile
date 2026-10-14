FROM mcr.microsoft.com/dotnet/aspnet:8.0 AS base
USER root
RUN apt-get update && apt-get install -y --no-install-recommends curl && rm -rf /var/lib/apt/lists/*
USER $APP_UID
WORKDIR /app
EXPOSE 8082

FROM mcr.microsoft.com/dotnet/sdk:8.0 AS build
ARG BUILD_CONFIGURATION=Release
WORKDIR /src
COPY ["Oxygen.API/Oxygen.API.csproj", "Oxygen.API/"]
COPY ["Oxygen.DTO/Oxygen.DTO.csproj", "Oxygen.DTO/"]
COPY ["Oxygen.Domain/Oxygen.Domain.csproj", "Oxygen.Domain/"]
COPY ["Oxygen.Infrastructure/Oxygen.Infrastructure.csproj", "Oxygen.Infrastructure/"]
COPY ["Oxygen.Proto/Oxygen.Proto.csproj", "Oxygen.Proto/"]
COPY ["Oxygen.Repository/Oxygen.Repository.csproj", "Oxygen.Repository/"]
COPY ["Oxygen.Service/Oxygen.Service.csproj", "Oxygen.Service/"]
RUN dotnet restore "Oxygen.API/Oxygen.API.csproj"
COPY . .
WORKDIR "/src/Oxygen.API"
# Publish compiles the project itself; a separate build step compiled it twice.
RUN dotnet publish "Oxygen.API.csproj" -c $BUILD_CONFIGURATION -o /app/publish --no-restore /p:UseAppHost=false

FROM base AS final
WORKDIR /app
COPY --from=build /app/publish .
ENV ASPNETCORE_URLS=http://+:8082
HEALTHCHECK --interval=30s --timeout=10s --start-period=40s --retries=3 \
  CMD curl -f http://localhost:8082/loan/health || exit 1
ENTRYPOINT ["dotnet", "Oxygen.API.dll"]
