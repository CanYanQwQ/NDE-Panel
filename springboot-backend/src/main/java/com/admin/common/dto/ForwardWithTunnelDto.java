package com.admin.common.dto;

import lombok.Data;

/**
 * <p>
 * 转发信息及关联隧道信息DTO
 * </p>
 *
 * @author QAQ
 * @since 2025-06-03
 */
@Data
public class ForwardWithTunnelDto {
    
    /**
     * 转发记录ID
     */
    private Long id;
    

    /**
     * 转发名称
     */
    private String name;
    

    /**
     * 入口端口
     */
    private Integer inPort;

    /**
     * 远程地址
     */
    private String remoteAddr;
    
    /**
     * 转发状态
     */
    private Integer status;
    
    /**
     * 创建时间
     */
    private Long createdTime;
    
    /**
     * 更新时间
     */
    private Long updatedTime;
    
    // 以下为隧道相关字段
    
    /**
     * 隧道名称
     */
    private String tunnelName;
    
    /**
     * 入口IP
     */
    private String inIp;

    private String userName;


    /**
     * 用户ID
     */
    private Integer userId;
    /**
     * 隧道ID
     */
    private Integer tunnelId;

    /**
     * 入站流量（字节）
     */
    private Long inFlow;
    
    /**
     * 出站流量（字节）
     */
    private Long outFlow;

    private String strategy;

    private Integer inx;

    private String interfaceName;

    /**
     * 是否是「搭协议/搭中转」时自动生成的转发。
     *
     * 这类转发是协议的内部管道(公网口 → 本机 sing-box 入站),用户在
     * 「协议管理」「中转」页面管它们,不该跟手工建的端口转发混在一起,
     * 所以转发管理页默认折叠隐藏,需要排障时再展开。
     */
    private Boolean protocolManaged;
}