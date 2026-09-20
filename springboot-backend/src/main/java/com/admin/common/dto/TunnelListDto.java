package com.admin.common.dto;

import lombok.Data;

@Data
public class TunnelListDto {

    private Integer id;

    private String name;
    
    /**
     * 入口IP
     */
    private String ip;
    
    /**
     * 入口节点端口起始范围
     */
    private Integer inNodePortSta;
    
    /**
     * 入口节点端口结束范围
     */
    private Integer inNodePortEnd;

    /**
     * 隧道类型（1-端口转发，2-隧道转发）
     */
    private Integer type;
    
    /**
     * 协议类型
     */
    private String protocol;

    /**
     * 入口机 / 出口机的节点ID。
     * 前端拿它找出「远程地址那台机器上已经搭了哪些协议」,好让用户点一下就填,不用手打 IP:端口。
     * 端口转发看入口机,隧道转发看出口机(远程地址是出口机去连的)。
     */
    private Long inNodeId;

    private Long outNodeId;
}
